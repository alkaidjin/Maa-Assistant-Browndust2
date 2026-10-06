//go:build windows

package launchgame

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	// 启动器写入的账号子键（本机实测 10000001，path=安装目录、execute=启动 exe 名）。
	neowizStarterKey = `Software\Neowiz\Browndust2Starter`
	uninstallRoot    = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
	uninstallRootWOW = `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`

	win32DPIAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

	// GetDriveType 返回值。
	driveRemovable = 2 // U 盘 / 移动硬盘（可移动介质）
	driveFixed     = 3 // 本地固定磁盘
)

// fallbackDrives 仅在盘符枚举 API 不可用时兜底。
var fallbackDrives = []string{"C:", "D:", "E:", "F:", "G:"}

var (
	win32User32                       = windows.NewLazySystemDLL("user32.dll")
	win32Kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	win32ProcEnumWindows              = win32User32.NewProc("EnumWindows")
	win32ProcIsWindowVisible          = win32User32.NewProc("IsWindowVisible")
	win32ProcGetClassNameW            = win32User32.NewProc("GetClassNameW")
	win32ProcGetWindowTextW           = win32User32.NewProc("GetWindowTextW")
	win32ProcGetClientRect            = win32User32.NewProc("GetClientRect")
	win32ProcIsWindow                 = win32User32.NewProc("IsWindow")
	win32ProcSetThreadDpiAwarenessCtx = win32User32.NewProc("SetThreadDpiAwarenessContext")
	win32ProcGetLogicalDrives         = win32Kernel32.NewProc("GetLogicalDrives")
	win32ProcGetDriveTypeW            = win32Kernel32.NewProc("GetDriveTypeW")
)

type win32Rect struct {
	Left, Top, Right, Bottom int32
}

var (
	enumOnce  sync.Once
	enumCb    uintptr
	enumFound uintptr
)

// findInstalledGame 是 Windows 平台检索链，按优先级返回首个命中的
// 游戏 exe 路径与来源标签：
//  1. Neowiz 启动器注册表（安装时选的任意盘/任意目录原样记录，最可靠）；
//  2. 全部固定/可移动盘上的默认安装路径；
//  3. 卸载信息表（尽力兜底）。
func findInstalledGame() (string, string) {
	if path := fromNeowizRegistry(); path != "" {
		return path, "neowiz-registry"
	}
	if path := fromDefaultLocations(); path != "" {
		return path, "default-location"
	}
	if path := fromUninstallRegistry(); path != "" {
		return path, "uninstall-registry"
	}
	return "", ""
}

// fromNeowizRegistry 读 HKCU\Software\Neowiz\Browndust2Starter\<id> 下的
// path + execute 组合成 exe 路径；execute 缺失时按默认进程名兜底。
// 兼容 execute 写成绝对路径的版本（此时直接使用，不再与 path 拼接）。
func fromNeowizRegistry() string {
	root, err := registry.OpenKey(registry.CURRENT_USER, neowizStarterKey, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		log.Debug().
			Err(err).
			Str("key", neowizStarterKey).
			Msg("launchgame: Neowiz starter key not readable")
		return ""
	}
	defer root.Close()

	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		log.Debug().
			Err(err).
			Str("key", neowizStarterKey).
			Msg("launchgame: failed to enumerate Neowiz starter sub keys")
		return ""
	}

	for _, id := range numericNames(names) {
		k, err := registry.OpenKey(root, id, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		dir, _, err := k.GetStringValue("path")
		if err != nil || strings.TrimSpace(dir) == "" {
			k.Close()
			continue
		}
		exe, _, err := k.GetStringValue("execute")
		k.Close()
		if err != nil || strings.TrimSpace(exe) == "" {
			exe = gameProcessName
		}
		var candidate string
		if filepath.IsAbs(exe) {
			candidate = exe
		} else {
			candidate = filepath.Join(dir, exe)
		}
		if isGameExe(candidate) {
			log.Info().
				Str("id", id).
				Str("path", candidate).
				Msg("launchgame: found game via Neowiz starter registry")
			return candidate
		}
	}
	return ""
}

// candidateDrives 枚举本机全部固定磁盘与可移动磁盘的盘符（A..Z 位图），
// 跳过光驱 / 网络盘 / RAM 盘；不做任何递归扫描，只拼固定安装目录。
// API 不可用时回退到 fallbackDrives。
func candidateDrives() []string {
	if err := win32ProcGetLogicalDrives.Find(); err != nil {
		return fallbackDrives
	}
	if err := win32ProcGetDriveTypeW.Find(); err != nil {
		return fallbackDrives
	}
	mask, _, _ := win32ProcGetLogicalDrives.Call()
	if mask == 0 {
		return fallbackDrives
	}
	var drives []string
	for bit := 0; bit < 26; bit++ {
		if mask&(1<<bit) == 0 {
			continue
		}
		root := string(rune('A'+bit)) + `:\`
		rootUTF16, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		driveType, _, _ := win32ProcGetDriveTypeW.Call(uintptr(unsafe.Pointer(rootUTF16)))
		if driveType != driveFixed && driveType != driveRemovable {
			continue
		}
		drives = append(drives, string(rune('A'+bit))+":")
	}
	if len(drives) == 0 {
		return fallbackDrives
	}
	return drives
}

// fromDefaultLocations 扫描每个固定/可移动盘上的默认安装位置。
// 注意 filepath.Join("C:", ...) 会得到 "C:Neowiz" 这种盘符相对路径，必须补 `\`。
func fromDefaultLocations() string {
	for _, drive := range candidateDrives() {
		candidate := filepath.Join(drive+`\`, "Neowiz", "Browndust2", "Browndust2_10000001", gameProcessName)
		if isGameExe(candidate) {
			log.Info().
				Str("path", candidate).
				Msg("launchgame: found game at default location")
			return candidate
		}
	}
	return ""
}

// fromUninstallRegistry 扫描卸载信息表（HKCU/HKLM 各含 WOW6432Node），
// DisplayName 含 browndust / brown dust / neowiz 的条目按三种字段取候选路径。
//
// 注意（2026-10-06 实测）：棕 2 游戏本体不单独登记卸载信息，卸载表里唯一
// 匹配条目 DisplayName=Browndust2 指向的是「启动器」
// （C:\ProgramData\Neowiz\Browndust2Starter\Browndust2Starter.exe），
// 因此本链在标准安装形态下不会命中，只对第三方重打包 / 把本体登记进
// 卸载表的特殊形态有意义；isGameExe 的严格文件名校验确保绝不会把
// Browndust2Starter.exe 误当游戏本体拉起。
func fromUninstallRegistry() string {
	for _, hive := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		for _, rootPath := range []string{uninstallRoot, uninstallRootWOW} {
			if path := scanUninstallRoot(hive, rootPath); path != "" {
				return path
			}
		}
	}
	return ""
}

func scanUninstallRoot(hive registry.Key, rootPath string) string {
	root, err := registry.OpenKey(hive, rootPath, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return ""
	}
	defer root.Close()

	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return ""
	}
	sort.Strings(names)

	for _, name := range names {
		k, err := registry.OpenKey(root, name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		displayName, _, _ := k.GetStringValue("DisplayName")
		if !matchUninstallName(displayName) {
			k.Close()
			continue
		}
		installLocation, _, _ := k.GetStringValue("InstallLocation")
		displayIcon, _, _ := k.GetStringValue("DisplayIcon")
		uninstallString, _, _ := k.GetStringValue("UninstallString")
		k.Close()

		var candidates []string
		if strings.TrimSpace(installLocation) != "" {
			candidates = append(candidates, filepath.Join(installLocation, gameProcessName))
		}
		if icon := stripArgs(displayIcon); icon != "" {
			candidates = append(candidates, icon)
		}
		if uninstall := stripArgs(uninstallString); uninstall != "" {
			candidates = append(candidates, filepath.Join(filepath.Dir(uninstall), gameProcessName))
		}
		for _, candidate := range candidates {
			if isGameExe(candidate) {
				log.Info().
					Str("key", rootPath+`\`+name).
					Str("path", candidate).
					Msg("launchgame: found game via uninstall registry")
				return candidate
			}
		}
	}
	return ""
}

func matchUninstallName(displayName string) bool {
	name := strings.ToLower(displayName)
	return strings.Contains(name, "browndust") ||
		strings.Contains(name, "brown dust") ||
		strings.Contains(name, "neowiz")
}

// stripArgs 处理 DisplayIcon / UninstallString 可能带的参数尾巴（如 `"C:\x.exe",0`）。
func stripArgs(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, `"`) {
		if end := strings.IndexByte(s[1:], '"'); end >= 0 {
			return s[1 : 1+end]
		}
		return strings.Trim(s, `"`)
	}
	if idx := strings.IndexAny(s, ", "); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// numericNames 过滤出数字子键名并按数值升序（启动器按账号 id 建子键）。
func numericNames(names []string) []string {
	var ids []int
	for _, name := range names {
		if n, err := strconv.Atoi(strings.TrimSpace(name)); err == nil {
			ids = append(ids, n)
		}
	}
	sort.Ints(ids)
	out := make([]string, len(ids))
	for i, n := range ids {
		out[i] = strconv.Itoa(n)
	}
	return out
}

// findGameWindow 枚举顶层窗口找游戏窗口，返回 (hwnd, true) 或 (0, false)。
// 回调必须用 sync.Once 只创建一次 —— windows.NewCallback 每次调用都分配一个
// 新的回调槽，轮询循环里重复创建会耗尽有限的回调池。
func findGameWindow() (uintptr, bool) {
	if err := win32ProcEnumWindows.Find(); err != nil {
		return 0, false
	}
	enumOnce.Do(func() {
		enumCb = windows.NewCallback(enumWndProc)
	})
	enumFound = 0
	win32ProcEnumWindows.Call(enumCb, 0)
	if enumFound != 0 {
		return enumFound, true
	}
	return 0, false
}

// enumWndProc 返回 0 停止枚举，返回 1 继续。
func enumWndProc(hwnd, lparam uintptr) uintptr {
	if ret, _, _ := win32ProcIsWindowVisible.Call(hwnd); ret == 0 {
		return 1
	}
	if !isGameWindow(hwnd) {
		return 1
	}
	enumFound = hwnd
	return 0
}

// isGameWindow 按类名精确 + 标题包含（忽略大小写）双条件判定，
// 与 interface.json 控制器的 class_regex / window_regex 同口径。
func isGameWindow(hwnd uintptr) bool {
	if windowClassNameString(hwnd) != windowClassName {
		return false
	}
	title := windowTitleString(hwnd)
	return strings.Contains(strings.ToLower(title), windowTitleKey)
}

func windowClassNameString(hwnd uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := win32ProcGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

func windowTitleString(hwnd uintptr) string {
	buf := make([]uint16, 512)
	n, _, _ := win32ProcGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// clientSize 返回窗口客户区尺寸（DPI aware 上下文下读取）。
func clientSize(hwnd uintptr) (int, int, error) {
	if ret, _, _ := win32ProcIsWindow.Call(hwnd); ret == 0 {
		return 0, 0, fmt.Errorf("invalid HWND: %d", hwnd)
	}

	restoreDPIContext := setDPIAwareWin32()
	defer restoreDPIContext()

	var rect win32Rect
	if ret, _, _ := win32ProcGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ret == 0 {
		return 0, 0, fmt.Errorf("GetClientRect failed for HWND: %d", hwnd)
	}
	w := int(rect.Right - rect.Left)
	h := int(rect.Bottom - rect.Top)
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("invalid client rect for HWND %d: %dx%d", hwnd, w, h)
	}
	return w, h, nil
}

func setDPIAwareWin32() func() {
	if err := win32ProcSetThreadDpiAwarenessCtx.Find(); err != nil {
		return func() {}
	}
	oldCtx, _, _ := win32ProcSetThreadDpiAwarenessCtx.Call(win32DPIAwarenessContextPerMonitorAwareV2)
	return func() {
		if oldCtx != 0 {
			win32ProcSetThreadDpiAwarenessCtx.Call(oldCtx)
		}
	}
}
