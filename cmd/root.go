package cmd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/manifoldco/promptui"
	"github.com/nicoxiang/geektime-downloader/internal/config"
	"github.com/nicoxiang/geektime-downloader/internal/geektime"
	"github.com/nicoxiang/geektime-downloader/internal/markdown"
	"github.com/nicoxiang/geektime-downloader/internal/pdf"
	"github.com/nicoxiang/geektime-downloader/internal/pkg/filenamify"
	"github.com/nicoxiang/geektime-downloader/internal/pkg/files"
	"github.com/nicoxiang/geektime-downloader/internal/video"
	"github.com/spf13/cobra"
	"golang.org/x/net/html"
)

var (
	phone                  string
	gcid                   string
	gcess                  string
	concurrency            int
	downloadFolder         string
	sp                     *spinner.Spinner
	selectedProduct        geektime.Course
	quality                string
	downloadComments       bool
	selectedProductType    productTypeSelectOption
	columnOutputType       int
	printPDFWaitSeconds    int
	printPDFTimeoutSeconds int
	interval               int
	productTypeOptions     []productTypeSelectOption
	geektimeClient         *geektime.Client
	isEnterprise           bool
	waitRand               = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type productTypeSelectOption struct {
	Index              int
	Text               string
	SourceType         int
	AcceptProductTypes []string
	needSelectArticle  bool
}

type articleOpsOption struct {
	Text  string
	Value int
}

func init() {
	downloadFolder = config.DefaultDownloadPath
	columnOutputType = 3 // 1(pdf) + 2(markdown) = 3
	concurrency = int(math.Ceil(float64(runtime.NumCPU()) / 2.0))
	sp = spinner.New(spinner.CharSets[4], 100*time.Millisecond)

	// 增加 PDF 相关的等待时间，避免超时
	printPDFWaitSeconds = 15     // 增加到 15 秒
	printPDFTimeoutSeconds = 120 // 增加到 120 秒
}

func setProductTypeOptions() {
	if isEnterprise {
		productTypeOptions = append(productTypeOptions, productTypeSelectOption{0, "训练营", 5, []string{"c44"}, true}) //custom source type, not use
	} else {
		productTypeOptions = append(productTypeOptions, productTypeSelectOption{0, "普通课程", 1, []string{"c1", "c3"}, true})
		productTypeOptions = append(productTypeOptions, productTypeSelectOption{1, "每日一课", 2, []string{"d"}, false})
		productTypeOptions = append(productTypeOptions, productTypeSelectOption{2, "公开课", 1, []string{"p35", "p29", "p30"}, true})
		productTypeOptions = append(productTypeOptions, productTypeSelectOption{3, "大厂案例", 4, []string{"q"}, false})
		productTypeOptions = append(productTypeOptions, productTypeSelectOption{4, "训练营", 5, []string{""}, true}) //custom source type, not use
		productTypeOptions = append(productTypeOptions, productTypeSelectOption{5, "其他", 1, []string{"x", "c6"}, true})
	}
}

func logError(msg string) {
	// 在下载目录下创建error.txt文件
	errorFile := filepath.Join(downloadFolder, "error.txt")
	f, err := os.OpenFile(errorFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("无法创建错误日志文件: %v\n", err)
		return
	}
	defer f.Close()

	// 添加时间戳
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMsg := fmt.Sprintf("[%s] %s\n", timestamp, msg)

	if _, err := f.WriteString(logMsg); err != nil {
		fmt.Printf("写入错误日志失败: %v\n", err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "geektime-downloader",
	Short: "Geektime-downloader is used to download geek time lessons",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		// 读取配置
		cfg, err := config.GetConfig()
		checkError(err)

		// 设置 cookies
		cookies := readCookiesFromConfig(cfg)

		fmt.Printf("正在验证登录...\n")
		fmt.Printf("使用的 Cookie 值:\n")
		fmt.Printf("GCID: %s\n", cfg.GCID)
		fmt.Printf("GCESS: %s\n", cfg.GCESS)

		// 验证登录
		if err := geektime.Auth(cookies); err != nil {
			fmt.Printf("登录验证失败: %v\n", err)
			// 尝试重新构建 cookie
			cookies = []*http.Cookie{
				{
					Name:     "GCID",
					Value:    cfg.GCID,
					Domain:   ".geekbang.org",
					Path:     "/",
					Secure:   true,
					HttpOnly: true,
				},
				{
					Name:     "GCESS",
					Value:    cfg.GCESS,
					Domain:   ".geekbang.org",
					Path:     "/",
					Secure:   true,
					HttpOnly: true,
				},
			}

			fmt.Printf("尝试重新验证登录...\n")
			if err := geektime.Auth(cookies); err != nil {
				checkError(err)
			}
		}
		fmt.Printf("登录验证成功\n")

		geektimeClient = geektime.NewClient(cookies)

		// 依次下载每个课程
		for _, courseID := range cfg.CourseIDs {
			func() {
				defer func() {
					if r := recover(); r != nil {
						errMsg := fmt.Sprintf("课程 %s 下载失败: %v", courseID, r)
						fmt.Printf("\n%s\n", errMsg)
						logError(errMsg)
					}
				}()

				id, err := strconv.Atoi(courseID)
				if err != nil {
					errMsg := fmt.Sprintf("无效的课程ID: %s, 跳过", courseID)
					fmt.Printf("%s\n", errMsg)
					logError(errMsg)
					return
				}

				fmt.Printf("\n正在获取课程信息, ID: %s\n", courseID)
				// 加载课程信息
				course, err := geektimeClient.CourseInfo(id)
				if err != nil {
					errMsg := fmt.Sprintf("获取课程信息失败: %s, 错误: %v", courseID, err)
					fmt.Printf("%s\n", errMsg)
					logError(errMsg)
					return
				}

				// 跳过视频课程
				if course.IsVideo {
					errMsg := fmt.Sprintf("课程 %s 为视频课程,跳过", course.Title)
					fmt.Printf("%s\n", errMsg)
					logError(errMsg)
					return
				}

				selectedProduct = course

				// 创建课程目录
				pdfDir, mdDir, err := mkDownloadProjectDir(downloadFolder, "", cfg.GCID, course.Title)
				if err != nil {
					errMsg := fmt.Sprintf("创建目录失败: %v", err)
					fmt.Printf("%s\n", errMsg)
					logError(errMsg)
					return
				}

				fmt.Printf("开始下载课程: %s\n", course.Title)
				total := len(course.Articles)
				var count int

				// 下载所有文章
				for _, article := range course.Articles {
					maxRetries := 5
					var downloadSuccess bool
					for retry := 0; retry < maxRetries; retry++ {
						if retry > 0 {
							fmt.Printf("\n正在重试第 %d 次下载 %s...\n", retry+1, article.Title)
							time.Sleep(time.Second * 5)
						}

						func() {
							defer func() {
								if r := recover(); r != nil {
									errMsg := fmt.Sprintf("文章 %s 下载出错: %v", article.Title, r)
									fmt.Printf("\n%s\n", errMsg)
									logError(errMsg)
								}
							}()

							skipped, err := downloadTextArticle(ctx, article, pdfDir, mdDir, false)
							if err != nil {
								errMsg := fmt.Sprintf("文章 %s 下载失败: %v", article.Title, err)
								fmt.Printf("\n%s\n", errMsg)
								logError(errMsg)
								return
							}
							if skipped {
								downloadSuccess = true
								return
							}
						}()

						if downloadSuccess {
							break
						}

						if retry < maxRetries-1 {
							waitRandomTime()
						}
					}

					if !downloadSuccess {
						errMsg := fmt.Sprintf("警告：文章 %s 下载失败", article.Title)
						fmt.Printf("\n%s\n", errMsg)
						logError(errMsg)
					}

					increaseDownloadedTextArticleCount(total, &count)

					// 每篇文章下载后等待一下，避免请求太频繁
					if count < total {
						waitRandomTime()
					}
				}

				fmt.Printf("\n课程 %s 下载完成\n", course.Title)
			}()

			// 课程之间增加等待时间
			time.Sleep(time.Second * 3)
		}

		fmt.Printf("\n所有课程下载任务完成！\n")
	},
}

func selectProductType(ctx context.Context) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "{{ `>` | red }} {{ .Text | red }}",
		Inactive: "{{ .Text }}",
	}
	prompt := promptui.Select{
		Label:        "请选择想要下载的产品类型",
		Items:        productTypeOptions,
		Templates:    templates,
		Size:         len(productTypeOptions),
		HideSelected: true,
		Stdout:       NoBellStdout,
	}
	index, _, err := prompt.Run()
	checkError(err)
	selectedProductType = productTypeOptions[index]
	letInputProductID(ctx)
}

func letInputProductID(ctx context.Context) {
	prompt := promptui.Prompt{
		Label: fmt.Sprintf("请输入%s的课程 ID", selectedProductType.Text),
		Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("课程 ID 不能为空")
			}
			if _, err := strconv.Atoi(s); err != nil {
				return errors.New("课程 ID 格式不合法")
			}
			return nil
		},
		HideEntered: true,
	}
	s, err := prompt.Run()
	checkError(err)

	// ignore, because checked before
	id, _ := strconv.Atoi(s)

	if selectedProductType.needSelectArticle {
		// choose download all or download specified article
		loadProduct(ctx, id)
		productOps(ctx)
	} else {
		// when product type is daily lesson or qconplus,
		// input id means product id
		// download video directly
		productInfo, err := geektimeClient.ProductInfo(id)
		checkError(err)

		if productInfo.Data.Info.Extra.Sub.AccessMask == 0 {
			fmt.Fprint(os.Stderr, "尚未购买该课程\n")
			letInputProductID(ctx)
		}

		if checkProductType(productInfo.Data.Info.Type) {
			pdfDir, _, err := mkDownloadProjectDir(downloadFolder, phone, gcid, productInfo.Data.Info.Title)
			checkError(err)

			err = video.DownloadArticleVideo(ctx,
				geektimeClient,
				productInfo.Data.Info.Article.ID,
				selectedProductType.SourceType,
				pdfDir,
				quality,
				concurrency)

			checkError(err)
		}
		letInputProductID(ctx)
	}
}

func loadProduct(ctx context.Context, productID int) {
	sp.Prefix = "[ 正在加载课程信息... ]"
	sp.Start()
	var p geektime.Course
	var err error
	if isUniversity() {
		// university don't need check product type
		// if input invalid id, access mark is 0
		p, err = geektimeClient.UniversityCourseInfo(productID)
	} else if isEnterprise {
		// TODO: check enterprise course type
		p, err = geektimeClient.EnterpriseCourseInfo(productID)
	} else {
		p, err = geektimeClient.CourseInfo(productID)
		if err == nil {
			c := checkProductType(p.Type)
			// if check product type fail, re-input product
			if !c {
				sp.Stop()
				letInputProductID(ctx)
			}
		}
	}

	if err != nil {
		sp.Stop()
		checkError(err)
	}
	sp.Stop()
	if !p.Access {
		fmt.Fprint(os.Stderr, "尚未购买该课程\n")
		letInputProductID(ctx)
	}
	selectedProduct = p
}

func productOps(ctx context.Context) {
	options := make([]articleOpsOption, 3)
	options[0] = articleOpsOption{"重新选择课程", 0}
	if isText() {
		options[1] = articleOpsOption{"下载当前专栏所有文章", 1}
		options[2] = articleOpsOption{"选择文章", 2}
	} else {
		options[1] = articleOpsOption{"下载所有视频", 1}
		options[2] = articleOpsOption{"选择视频", 2}
	}
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "{{ `>` | red }} {{ .Text | red }}",
		Inactive: "{{if eq .Value 0}} {{ .Text | green }} {{else}} {{ .Text }} {{end}}",
	}
	prompt := promptui.Select{
		Label:        fmt.Sprintf("当前选中的专栏为: %s, 请继续选择：", selectedProduct.Title),
		Items:        options,
		Templates:    templates,
		Size:         len(options),
		HideSelected: true,
		Stdout:       NoBellStdout,
	}
	index, _, err := prompt.Run()
	checkError(err)

	switch index {
	case 0:
		selectProductType(ctx)
	case 1:
		handleDownloadAll(ctx)
	case 2:
		selectArticle(ctx)
	}
}

func selectArticle(ctx context.Context) {
	items := []geektime.Article{
		{
			AID:   -1,
			Title: "返回上一级",
		},
	}
	items = append(items, selectedProduct.Articles...)
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "{{ `>` | red }} {{ .Title | red }}",
		Inactive: "{{if eq .AID -1}} {{ .Title | green }} {{else}} {{ .Title }} {{end}}",
	}
	prompt := promptui.Select{
		Label:        "请选择文章: ",
		Items:        items,
		Templates:    templates,
		Size:         20,
		HideSelected: true,
		CursorPos:    0,
		Stdout:       NoBellStdout,
	}
	index, _, err := prompt.Run()
	checkError(err)
	handleSelectArticle(ctx, index)
}

func handleSelectArticle(ctx context.Context, index int) {
	if index == 0 {
		productOps(ctx)
	}
	a := selectedProduct.Articles[index-1]

	// 创建目录
	pdfDir, mdDir, err := mkDownloadProjectDir(downloadFolder, phone, gcid, selectedProduct.Title)
	checkError(err)

	// 修改 downloadArticle 调用
	downloadArticle(ctx, a, pdfDir, mdDir)
	fmt.Printf("\r%s 下载完成", a.Title)
	time.Sleep(time.Second)
	selectArticle(ctx)
}

func handleDownloadAll(ctx context.Context) {
	// 创建目录
	pdfDir, mdDir, err := mkDownloadProjectDir(downloadFolder, phone, gcid, selectedProduct.Title)
	checkError(err)

	if isText() {
		fmt.Printf("正在下载专栏 《%s》 中的所有文章\n", selectedProduct.Title)
		total := len(selectedProduct.Articles)
		var count int

		for _, article := range selectedProduct.Articles {
			skipped, err := downloadTextArticle(ctx, article, pdfDir, mdDir, false)
			if err != nil {
				fmt.Printf("下载文章失败: %v\n", err)
				continue
			}

			increaseDownloadedTextArticleCount(total, &count)

			if !skipped {
				waitRandomTime()
			}
		}
	} else {
		for _, article := range selectedProduct.Articles {
			skipped := downloadVideoArticle(ctx, article, pdfDir, false)
			if !skipped {
				waitRandomTime()
			}
		}
	}
	selectProductType(ctx)
}

func increaseDownloadedTextArticleCount(total int, i *int) {
	*i++
	fmt.Printf("\r已完成下载%d/%d", *i, total)
}

func downloadArticle(ctx context.Context, article geektime.Article, pdfDir, mdDir string) {
	if isText() {
		sp.Prefix = fmt.Sprintf("[ 正在下载 《%s》... ]", article.Title)
		sp.Start()
		defer sp.Stop()
		skipped, err := downloadTextArticle(ctx, article, pdfDir, mdDir, true)
		if err != nil {
			fmt.Printf("下载文章失败: %v\n", err)
		}
		if !skipped {
			waitRandomTime()
		}
	} else {
		downloadVideoArticle(ctx, article, pdfDir, true)
	}
}

func downloadTextArticle(ctx context.Context, article geektime.Article, pdfDir, mdDir string, overwrite bool) (bool, error) {
	needDownloadPDF := columnOutputType&1 == 1
	needDownloadMD := (columnOutputType>>1)&1 == 1
	skipped := true

	articleInfo, err := geektimeClient.V1ArticleInfo(article.AID)
	if err != nil {
		return false, fmt.Errorf("获取文章信息失败: %v", err)
	}

	hasVideo, videoURL := getVideoURLFromArticleContent(articleInfo.Data.ArticleContent)
	if hasVideo && videoURL != "" {
		err = video.DownloadMP4(ctx, article.Title, pdfDir, []string{videoURL}, overwrite)
		if err != nil {
			return false, fmt.Errorf("下载视频失败: %v", err)
		}
	}

	if len(articleInfo.Data.InlineVideoSubtitles) > 0 {
		videoURLs := make([]string, len(articleInfo.Data.InlineVideoSubtitles))
		for i, v := range articleInfo.Data.InlineVideoSubtitles {
			videoURLs[i] = v.VideoURL
		}
		err = video.DownloadMP4(ctx, article.Title, pdfDir, videoURLs, overwrite)
		if err != nil {
			return false, fmt.Errorf("下载内嵌视频失败: %v", err)
		}
	}

	if needDownloadPDF {
		innerSkipped, err := pdf.PrintArticlePageToPDF(ctx,
			article.AID,
			pdfDir,
			article.Title,
			geektimeClient.Cookies,
			downloadComments,
			printPDFWaitSeconds,
			printPDFTimeoutSeconds,
			overwrite,
		)
		if err != nil {
			return false, fmt.Errorf("生成PDF失败: %v", err)
		}
		if !innerSkipped {
			skipped = false
		}
	}

	if needDownloadMD {
		innerSkipped, err := markdown.Download(ctx,
			articleInfo.Data.ArticleContent,
			article.Title,
			mdDir,
			article.AID,
			overwrite)
		if err != nil {
			return false, fmt.Errorf("生成Markdown失败: %v", err)
		}
		if !innerSkipped {
			skipped = false
		}
	}

	return skipped, nil
}

func downloadVideoArticle(ctx context.Context, article geektime.Article, projectDir string, overwrite bool) bool {
	dir := projectDir
	var err error
	// add sub dir
	if article.SectionTitle != "" {
		dir, err = mkDownloadProjectSectionDir(projectDir, article.SectionTitle)
		checkError(err)
	}

	fileName := filenamify.Filenamify(article.Title) + video.TSExtension
	fullPath := filepath.Join(dir, fileName)
	if files.CheckFileExists(fullPath) && !overwrite {
		return true
	}

	if isUniversity() {
		err = video.DownloadUniversityVideo(ctx, geektimeClient, article.AID, selectedProduct, dir, quality, concurrency)
		checkError(err)
	} else if isEnterprise {
		err = video.DownloadEnterpriseArticleVideo(ctx, geektimeClient, article.AID, dir, quality, concurrency)
		checkError(err)
	} else {
		err = video.DownloadArticleVideo(ctx, geektimeClient, article.AID, selectedProductType.SourceType, dir, quality, concurrency)
		checkError(err)
	}
	return false
}

func isText() bool {
	return !selectedProduct.IsVideo
}

func isUniversity() bool {
	return selectedProductType.Index == 4 && !isEnterprise
}

func readCookiesFromInput() []*http.Cookie {
	oneyear := time.Now().Add(180 * 24 * time.Hour)
	cookies := make([]*http.Cookie, 2)
	m := make(map[string]string, 2)
	m[geektime.GCID] = gcid
	m[geektime.GCESS] = gcess
	c := 0
	for k, v := range m {
		cookies[c] = &http.Cookie{
			Name:     k,
			Value:    v,
			Domain:   geektime.GeekBangCookieDomain,
			HttpOnly: true,
			Expires:  oneyear,
		}
		c++
	}
	return cookies
}

func mkDownloadProjectDir(downloadFolder, phone, gcid, projectName string) (string, string, error) {
	// 创建 PDF 目录
	pdfPath := filepath.Join(downloadFolder, "pdf", filenamify.Filenamify(projectName))
	if err := os.MkdirAll(pdfPath, os.ModePerm); err != nil {
		return "", "", err
	}

	// 创建 Markdown 目录
	mdPath := filepath.Join(downloadFolder, "markdown", filenamify.Filenamify(projectName))
	if err := os.MkdirAll(mdPath, os.ModePerm); err != nil {
		return "", "", err
	}

	return pdfPath, mdPath, nil
}

func mkDownloadProjectSectionDir(downloadFolder, sectionName string) (string, error) {
	path := filepath.Join(downloadFolder, filenamify.Filenamify(sectionName))
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return "", err
	}
	return path, nil
}

func checkProductType(productType string) bool {
	for _, pt := range selectedProductType.AcceptProductTypes {
		if pt == productType {
			return true
		}
	}
	fmt.Fprint(os.Stderr, "\r输入的课程 ID 有误\n")
	return false
}

// Sometime video exist in article content, see issue #104
// <p>
// <video poster="https://static001.geekbang.org/resource/image/6a/f7/6ada085b44eddf37506b25ad188541f7.jpg" preload="none" controls="">
// <source src="https://media001.geekbang.org/customerTrans/fe4a99b62946f2c31c2095c167b26f9c/30d99c0d-16d14089303-0000-0000-01d-dbacd.mp4" type="video/mp4">
// <source src="https://media001.geekbang.org/2ce11b32e3e740ff9580185d8c972303/a01ad13390fe4afe8856df5fb5d284a2-f2f547049c69fa0d4502ab36d42ea2fa-sd.m3u8" type="application/x-mpegURL">
// <source src="https://media001.geekbang.org/2ce11b32e3e740ff9580185d8c972303/a01ad13390fe4afe8856df5fb5d284a2-2528b0077e78173fd8892de4d7b8c96d-hd.m3u8" type="application/x-mpegURL"></video>
// </p>
func getVideoURLFromArticleContent(content string) (hasVideo bool, videoURL string) {
	if !strings.Contains(content, "<video") || !strings.Contains(content, "<source") {
		return false, ""
	}
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return false, ""
	}
	hasVideo, videoURL = false, ""
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "video" {
			hasVideo = true
		}
		if n.Type == html.ElementNode && n.Data == "source" {
			for _, a := range n.Attr {
				if a.Key == "src" && hasVideo && strings.HasSuffix(a.Val, ".mp4") {
					videoURL = a.Val
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return hasVideo, videoURL
}

// waitRandomTime wait interval seconds of time plus a 2000ms max jitter
func waitRandomTime() {
	randomMillis := interval*1000 + waitRand.Intn(2000)
	time.Sleep(time.Duration(randomMillis) * time.Millisecond)
}

// Execute ...
func Execute() {
	ctx := context.Background()

	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		checkError(err)
	}
}

func readCookiesFromConfig(cfg *config.Config) []*http.Cookie {
	oneyear := time.Now().Add(180 * 24 * time.Hour)
	cookies := make([]*http.Cookie, 2)

	cookies[0] = &http.Cookie{
		Name:     geektime.GCID,
		Value:    cfg.GCID,
		Domain:   ".geekbang.org",
		Path:     "/",
		HttpOnly: true,
		Expires:  oneyear,
	}

	cookies[1] = &http.Cookie{
		Name:     geektime.GCESS,
		Value:    cfg.GCESS,
		Domain:   ".geekbang.org",
		Path:     "/",
		HttpOnly: true,
		Expires:  oneyear,
	}

	return cookies
}
