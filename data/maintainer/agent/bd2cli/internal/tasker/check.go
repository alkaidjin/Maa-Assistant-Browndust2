package tasker

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"unsafe"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/MaaXYZ/maa-framework-go/v4/controller/win32"
)

// InputCheckResult 是"体检"结果：只截图、不点击，用来验证眼睛有没有睁开。
type InputCheckResult struct {
	Window    uintptr `json:"window"`
	Inited    bool    `json:"inited"`
	Connected bool    `json:"connected"`
	Screencap bool    `json:"screencap"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	MeanLuma  float64 `json:"mean_luma"`  // 平均亮度 0-255，用于判断是否黑屏
	PNG       string  `json:"png,omitempty"`
	Error     string  `json:"error,omitempty"`
}

// meanLuma 抽样计算平均亮度（每 8 像素取一个点）。
func meanLuma(img image.Image) float64 {
	b := img.Bounds()
	var sum float64
	var n int
	for y := b.Min.Y; y < b.Max.Y; y += 8 {
		for x := b.Min.X; x < b.Max.X; x += 8 {
			r, g, bl, _ := img.At(x, y).RGBA()
			sum += 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// CheckInput 建控制器、连窗口、截一张图存到 pngPath。**不做任何点击**。
func CheckInput(cfg Config, hwnd unsafe.Pointer, pngPath string) InputCheckResult {
	r := InputCheckResult{}
	if hwnd == nil {
		r.Error = fmt.Sprintf("找不到游戏窗口（class=%q title=%q）", cfg.Class, cfg.WindowTitle)
		return r
	}
	r.Window = uintptr(hwnd)

	if !maa.IsInited() {
		if err := maa.Init(maa.WithLibDir(filepath.Join(cfg.Root, "maafw"))); err != nil {
			r.Error = fmt.Sprintf("初始化 MaaFramework 失败: %v", err)
			return r
		}
	}
	r.Inited = true

	screencap, err := win32.ParseScreencapMethod(cfg.ScreencapMethod)
	if err != nil {
		r.Error = fmt.Sprintf("截图方式 %q 非法: %v", cfg.ScreencapMethod, err)
		return r
	}
	mouse, err := win32.ParseInputMethod(cfg.MouseMethod)
	if err != nil {
		r.Error = fmt.Sprintf("鼠标方式 %q 非法: %v", cfg.MouseMethod, err)
		return r
	}
	keyboard, err := win32.ParseInputMethod(cfg.KeyboardMethod)
	if err != nil {
		r.Error = fmt.Sprintf("键盘方式 %q 非法: %v", cfg.KeyboardMethod, err)
		return r
	}

	ctrl, err := maa.NewWin32Controller(hwnd, screencap, mouse, keyboard)
	if err != nil {
		r.Error = fmt.Sprintf("创建 Win32 控制器失败: %v", err)
		return r
	}
	conn := ctrl.PostConnect()
	conn.Wait()
	if !conn.Success() {
		r.Error = "连接游戏窗口失败"
		return r
	}
	r.Connected = true

	job := ctrl.PostScreencap()
	job.Wait()
	if !job.Success() {
		r.Error = "截图失败（可能被独占/黑屏，或截图方式不适配）"
		return r
	}
	r.Screencap = true

	var img image.Image
	if img, err = ctrl.CacheImage(); err != nil {
		r.Error = fmt.Sprintf("取图失败: %v", err)
		return r
	}
	b := img.Bounds()
	r.Width, r.Height = b.Dx(), b.Dy()
	r.MeanLuma = meanLuma(img)

	if pngPath != "" {
		if err := os.MkdirAll(filepath.Dir(pngPath), 0o755); err != nil {
			r.Error = fmt.Sprintf("建目录失败: %v", err)
			return r
		}
		f, err := os.Create(pngPath)
		if err != nil {
			r.Error = fmt.Sprintf("写图片失败: %v", err)
			return r
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			r.Error = fmt.Sprintf("编码 PNG 失败: %v", err)
			return r
		}
		r.PNG = pngPath
	}
	return r
}
