//go:build windows && !beamcast_cli

package main

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	CS_HREDRAW          = 0x0002
	CS_VREDRAW          = 0x0001
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	CW_USEDEFAULT       = ^uintptr(0x7fffffff)
	SW_SHOW             = 5
	WM_CREATE           = 0x0001
	WM_DESTROY          = 0x0002
	WM_SIZE             = 0x0005
	WM_PAINT            = 0x000F
	WM_CLOSE            = 0x0010
	WM_COMMAND          = 0x0111
	WM_BENCH_DONE       = 0x8001
	WM_TIMER            = 0x0113
	WM_KEYDOWN          = 0x0100
	WM_LBUTTONDOWN      = 0x0201
	WM_LBUTTONUP        = 0x0202
	WM_RBUTTONDOWN      = 0x0204
	WM_RBUTTONUP        = 0x0205
	WM_MOUSEMOVE        = 0x0200
	WM_MOUSEWHEEL       = 0x020A
	WM_ERASEBKGND       = 0x0014
	IDC_ARROW           = 32512
	IDI_APPLICATION     = 32512
	MF_STRING           = 0x0000
	MF_SEPARATOR        = 0x0800
	MF_POPUP            = 0x0010
	MF_CHECKED          = 0x0008
	MF_UNCHECKED        = 0x0000
	MB_OK               = 0x00000000
	MB_ICONINFORMATION  = 0x00000040
	MB_ICONERROR        = 0x00000010
	OFN_FILEMUSTEXIST   = 0x00001000
	OFN_PATHMUSTEXIST   = 0x00000800
	OFN_HIDEREADONLY    = 0x00000004
	OFN_OVERWRITEPROMPT = 0x00000002
	DIB_RGB_COLORS      = 0
	SRCCOPY             = 0x00CC0020
	HALFTONE            = 4
	TRANSPARENT         = 1
	VK_F5               = 0x74
	VK_ESCAPE           = 0x1B
	VK_CONTROL          = 0x11
	VK_SHIFT            = 0x10
	VK_DELETE           = 0x2E
	VK_OEM_4            = 0xDB
	VK_OEM_6            = 0xDD
	MK_LBUTTON          = 0x0001
	MK_RBUTTON          = 0x0002
)

const (
	ID_FILE_NEW             = 100
	ID_FILE_OPEN            = 101
	ID_FILE_SAVE_PPM        = 102
	ID_FILE_SAVE_BMP        = 103
	ID_FILE_EXIT            = 104
	ID_FILE_SAVE_PFM        = 105
	ID_SCENE_SHOWCASE       = 120
	ID_SCENE_CORNELL        = 121
	ID_SCENE_STRESS         = 122
	ID_RENDER_START         = 140
	ID_RENDER_CANCEL        = 141
	ID_QUALITY_DRAFT        = 150
	ID_QUALITY_PREVIEW      = 151
	ID_QUALITY_BALANCED     = 152
	ID_QUALITY_HIGH         = 153
	ID_QUALITY_ULTRA        = 154
	ID_TOGGLE_DENOISE       = 160
	ID_TOGGLE_AUTORENDER    = 161
	ID_RESOLUTION_540P      = 162
	ID_RESOLUTION_720P      = 163
	ID_RESOLUTION_900P      = 164
	ID_RESOLUTION_1080P     = 165
	ID_BACKEND_AUTO         = 166
	ID_BACKEND_CPU          = 167
	ID_BACKEND_GPU          = 168
	ID_UPSCALE_OFF          = 169
	ID_UPSCALE_UQ           = 170
	ID_UPSCALE_BALANCED     = 171
	ID_UPSCALE_PERF         = 172
	ID_CAMERA_RESET         = 180
	ID_CAMERA_LEFT          = 181
	ID_CAMERA_RIGHT         = 182
	ID_CAMERA_UP            = 183
	ID_CAMERA_DOWN          = 184
	ID_CAMERA_NEAR          = 185
	ID_CAMERA_FAR           = 186
	ID_VIEW_LEFT            = 200
	ID_VIEW_RIGHT           = 201
	ID_VIEW_RESET           = 202
	ID_HELP_ABOUT           = 220
	ID_RESOLUTION_1440P     = 230
	ID_EXPOSURE_MINUS       = 231
	ID_EXPOSURE_ZERO        = 232
	ID_EXPOSURE_PLUS        = 233
	ID_GAMMA_20             = 234
	ID_GAMMA_22             = 235
	ID_GAMMA_24             = 236
	ID_PRESENT_15           = 237
	ID_PRESENT_30           = 238
	ID_PRESENT_60           = 239
	ID_SPEED_FINE           = 240
	ID_SPEED_NORMAL         = 241
	ID_SPEED_FAST           = 242
	ID_FXAA_OFF             = 250
	ID_FXAA_LOW             = 251
	ID_FXAA_MED             = 252
	ID_FXAA_HIGH            = 253
	ID_TAA_OFF              = 254
	ID_TAA_LOW              = 255
	ID_TAA_MED              = 256
	ID_TAA_HIGH             = 257
	ID_TSAA_OFF             = 258
	ID_TSAA_LOW             = 259
	ID_TSAA_MED             = 260
	ID_TSAA_HIGH            = 261
	ID_TXAA_OFF             = 262
	ID_TXAA_LOW             = 263
	ID_TXAA_MED             = 264
	ID_TXAA_HIGH            = 265
	ID_MSAA_1               = 266
	ID_MSAA_2               = 267
	ID_MSAA_4               = 268
	ID_MSAA_8               = 269
	ID_HBAO_OFF             = 270
	ID_HBAO_LOW             = 271
	ID_HBAO_MED             = 272
	ID_HBAO_HIGH            = 273
	ID_HBAOP_OFF            = 274
	ID_HBAOP_LOW            = 275
	ID_HBAOP_MED            = 276
	ID_HBAOP_HIGH           = 277
	ID_BLOOM_OFF            = 278
	ID_BLOOM_LOW            = 279
	ID_BLOOM_MED            = 280
	ID_BLOOM_HIGH           = 281
	ID_HDR_OFF              = 282
	ID_HDR_REINHARD         = 283
	ID_HDR_FILMIC           = 284
	ID_HDR_ACES             = 285
	ID_LIVE_PREVIEW         = 286
	ID_OBJECT_EDIT          = 287
	ID_OBJECT_DIFFUSE       = 288
	ID_OBJECT_METAL         = 289
	ID_OBJECT_GLASS         = 290
	ID_OBJECT_LIGHT         = 291
	ID_OBJECT_CUBE          = 292
	ID_INTEGRATOR_PATH      = 293
	ID_INTEGRATOR_RECURSIVE = 294
	ID_INTEGRATOR_PHOTON    = 295
	ID_PHOTON_OFF           = 296
	ID_PHOTON_LOW           = 297
	ID_PHOTON_MED           = 298
	ID_PHOTON_HIGH          = 299
	ID_PHOTON_ULTRA         = 300
	ID_DEBUG_BEAUTY         = 301
	ID_DEBUG_ALBEDO         = 302
	ID_DEBUG_NORMAL         = 303
	ID_DEBUG_DEPTH          = 304
	ID_DEBUG_AO             = 305
	ID_DEBUG_HEAT           = 306
	ID_TILE_8               = 307
	ID_TILE_16              = 308
	ID_TILE_32              = 309
	ID_WORKERS_AUTO         = 310
	ID_WORKERS_4            = 311
	ID_WORKERS_8            = 312
	ID_ADAPTIVE_SAMPLING    = 313
	ID_SAMPLER_RANDOM       = 314
	ID_SAMPLER_HALTON       = 315
	ID_SAMPLER_R2           = 316
	ID_MIS_TOGGLE           = 317
	ID_POWER_LIGHT_TOGGLE   = 318
	ID_CLAMP_OFF            = 319
	ID_CLAMP_12             = 320
	ID_CLAMP_24             = 321
	ID_CLAMP_48             = 322
	ID_RR_2                 = 323
	ID_RR_4                 = 324
	ID_RR_6                 = 325
	ID_ADAPT_LOOSE          = 326
	ID_ADAPT_MED            = 327
	ID_ADAPT_TIGHT          = 328
	ID_GPU_COMPAT           = 329
	ID_BVH_MEDIAN           = 330
	ID_BVH_SAH8             = 331
	ID_BVH_SAH16            = 332
	ID_BVH_SAH32            = 333
	ID_LEAF_2               = 334
	ID_LEAF_4               = 335
	ID_LEAF_8               = 336
	ID_TILE_SCANLINE        = 337
	ID_TILE_CENTER          = 338
	ID_PROGRESSIVE_TOGGLE   = 339
	ID_PUBLISH_15           = 340
	ID_PUBLISH_30           = 341
	ID_PUBLISH_60           = 342
	ID_HYPER_LATENCY        = 343
	ID_HYPER_THROUGHPUT     = 344
	ID_HYPER_FINAL          = 345
	ID_WORKERS_1            = 346
	ID_WORKERS_2            = 347
	ID_WORKERS_16           = 348
	ID_WORKERS_32           = 349
	ID_HYPER_MULTICORE      = 350
	ID_BENCH_QUICK          = 351
	ID_BENCH_FULL           = 352
	ID_BENCH_SCALING        = 353
	ID_BENCH_POST           = 354
	ID_BENCH_INTEGRATORS    = 355
	ID_BENCH_GPU            = 356
	ID_BENCH_SHOW_LAST      = 357
	ID_GPU_DATAFLOW         = 358
	ID_QUALITY_REALISM      = 359
	ID_PANEL_QUALITY        = 360
)

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}
type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}
type OPENFILENAME struct {
	LStructSize       uint32
	HwndOwner         uintptr
	HInstance         uintptr
	LpstrFilter       *uint16
	LpstrCustomFilter *uint16
	NMaxCustFilter    uint32
	NFilterIndex      uint32
	LpstrFile         *uint16
	NMaxFile          uint32
	LpstrFileTitle    *uint16
	NMaxFileTitle     uint32
	LpstrInitialDir   *uint16
	LpstrTitle        *uint16
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       *uint16
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    *uint16
	PvReserved        uintptr
	DwReserved        uint32
	FlagsEx           uint32
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")

	pRegisterClassExW = user32.NewProc("RegisterClassExW")
	pCreateWindowExW  = user32.NewProc("CreateWindowExW")
	pShowWindow       = user32.NewProc("ShowWindow")
	pUpdateWindow     = user32.NewProc("UpdateWindow")
	pGetMessageW      = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessageW = user32.NewProc("DispatchMessageW")
	pDefWindowProcW   = user32.NewProc("DefWindowProcW")
	pPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pPostMessageW     = user32.NewProc("PostMessageW")
	pBeginPaint       = user32.NewProc("BeginPaint")
	pEndPaint         = user32.NewProc("EndPaint")
	pInvalidateRect   = user32.NewProc("InvalidateRect")
	pGetClientRect    = user32.NewProc("GetClientRect")
	pLoadCursorW      = user32.NewProc("LoadCursorW")
	pLoadIconW        = user32.NewProc("LoadIconW")
	pCreateMenu       = user32.NewProc("CreateMenu")
	pCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	pAppendMenuW      = user32.NewProc("AppendMenuW")
	pSetMenu          = user32.NewProc("SetMenu")
	pMessageBoxW      = user32.NewProc("MessageBoxW")
	pSetTimer         = user32.NewProc("SetTimer")
	pKillTimer        = user32.NewProc("KillTimer")
	pDestroyWindow    = user32.NewProc("DestroyWindow")
	pSetCapture       = user32.NewProc("SetCapture")
	pGetKeyState      = user32.NewProc("GetKeyState")
	pReleaseCapture   = user32.NewProc("ReleaseCapture")
	pFillRect         = user32.NewProc("FillRect")
	pFrameRect        = user32.NewProc("FrameRect")

	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pTextOutW               = gdi32.NewProc("TextOutW")
	pStretchDIBits          = gdi32.NewProc("StretchDIBits")
	pSetStretchBltMode      = gdi32.NewProc("SetStretchBltMode")
	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pDeleteDC               = gdi32.NewProc("DeleteDC")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	pGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")
)

func ptr(s string) *uint16     { p, _ := syscall.UTF16PtrFromString(s); return p }
func wchars(s string) []uint16 { return append(utf16.Encode([]rune(s)), 0) }
func multiW(parts ...string) []uint16 {
	out := []uint16{}
	for _, s := range parts {
		out = append(out, utf16.Encode([]rune(s))...)
		out = append(out, 0)
	}
	out = append(out, 0)
	return out
}
func rgb(r, g, b byte) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }
func loword(v uintptr) int     { return int(v & 0xffff) }
func signedLo(v uintptr) int   { return int(int16(v & 0xffff)) }
func signedHi(v uintptr) int   { return int(int16((v >> 16) & 0xffff)) }

func fillRect(hdc uintptr, r RECT, color uintptr) {
	brush, _, _ := pCreateSolidBrush.Call(color)
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
	pDeleteObject.Call(brush)
}
func textOut(hdc uintptr, x, y int, s string, color uintptr) {
	u := wchars(s)
	pSetTextColor.Call(hdc, color)
	pSetBkMode.Call(hdc, TRANSPARENT)
	pTextOutW.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1))
}
func drawButton(hdc uintptr, r RECT, label string, enabled bool) {
	bg := rgb(48, 53, 62)
	fg := rgb(232, 235, 240)
	if !enabled {
		bg = rgb(38, 41, 46)
		fg = rgb(120, 124, 132)
	}
	fillRect(hdc, r, bg)
	border, _, _ := pCreateSolidBrush.Call(rgb(82, 88, 99))
	pFrameRect.Call(hdc, uintptr(unsafe.Pointer(&r)), border)
	pDeleteObject.Call(border)
	textOut(hdc, int(r.Left)+12, int(r.Top)+8, label, fg)
}
func inside(r RECT, x, y int) bool {
	return x >= int(r.Left) && x < int(r.Right) && y >= int(r.Top) && y < int(r.Bottom)
}

func appendMenu(menu uintptr, flags uintptr, id uintptr, label string) {
	var p *uint16
	if label != "" {
		p = ptr(label)
	}
	pAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(p)))
}

func buildMenu() uintptr {
	bar, _, _ := pCreateMenu.Call()
	file, _, _ := pCreatePopupMenu.Call()
	appendMenu(file, MF_STRING, ID_FILE_NEW, "New showcase\tCtrl+N")
	appendMenu(file, MF_STRING, ID_FILE_OPEN, "Open OBJ...\tCtrl+O")
	appendMenu(file, MF_SEPARATOR, 0, "")
	appendMenu(file, MF_STRING, ID_FILE_SAVE_PPM, "Save displayed render as PPM...\tCtrl+S")
	appendMenu(file, MF_STRING, ID_FILE_SAVE_BMP, "Save displayed render as BMP...")
	appendMenu(file, MF_STRING, ID_FILE_SAVE_PFM, "Save linear HDR as PFM...")
	appendMenu(file, MF_SEPARATOR, 0, "")
	appendMenu(file, MF_STRING, ID_FILE_EXIT, "Exit")
	appendMenu(bar, MF_POPUP, file, "File")
	scene, _, _ := pCreatePopupMenu.Call()
	appendMenu(scene, MF_STRING, ID_SCENE_SHOWCASE, "Material showcase")
	appendMenu(scene, MF_STRING, ID_SCENE_CORNELL, "Cornell-like room")
	appendMenu(scene, MF_STRING, ID_SCENE_STRESS, "Stress grid")
	appendMenu(bar, MF_POPUP, scene, "Scene")
	render, _, _ := pCreatePopupMenu.Call()
	appendMenu(render, MF_STRING, ID_RENDER_START, "Start render\tF5")
	appendMenu(render, MF_STRING, ID_RENDER_CANCEL, "Cancel\tEsc")
	q, _, _ := pCreatePopupMenu.Call()
	appendMenu(q, MF_STRING, ID_QUALITY_DRAFT, "Draft")
	appendMenu(q, MF_STRING, ID_QUALITY_PREVIEW, "Preview")
	appendMenu(q, MF_STRING, ID_QUALITY_BALANCED, "Balanced")
	appendMenu(q, MF_STRING, ID_QUALITY_HIGH, "High")
	appendMenu(q, MF_STRING, ID_QUALITY_ULTRA, "Ultra")
	appendMenu(q, MF_STRING, ID_QUALITY_REALISM, "Ultra Realism")
	appendMenu(render, MF_POPUP, q, "Quality")
	res, _, _ := pCreatePopupMenu.Call()
	appendMenu(res, MF_STRING, ID_RESOLUTION_540P, "640 x 360")
	appendMenu(res, MF_STRING, ID_RESOLUTION_720P, "1280 x 720")
	appendMenu(res, MF_STRING, ID_RESOLUTION_900P, "1600 x 900")
	appendMenu(res, MF_STRING, ID_RESOLUTION_1080P, "1920 x 1080")
	appendMenu(res, MF_STRING, ID_RESOLUTION_1440P, "2560 x 1440")
	appendMenu(render, MF_POPUP, res, "Resolution")
	backend, _, _ := pCreatePopupMenu.Call()
	appendMenu(backend, MF_STRING, ID_BACKEND_AUTO, "Auto")
	appendMenu(backend, MF_STRING, ID_BACKEND_CPU, "CPU")
	appendMenu(backend, MF_STRING, ID_BACKEND_GPU, "OpenCL GPU (experimental)")
	appendMenu(backend, MF_SEPARATOR, 0, "")
	appendMenu(backend, MF_STRING, ID_GPU_COMPAT, "Apply GPU compatibility transport preset")
	appendMenu(backend, MF_STRING, ID_GPU_DATAFLOW, "Apply GPU throughput / no-guide preset")
	appendMenu(render, MF_POPUP, backend, "Backend")
	integrator, _, _ := pCreatePopupMenu.Call()
	appendMenu(integrator, MF_STRING, ID_INTEGRATOR_PATH, "Path tracing")
	appendMenu(integrator, MF_STRING, ID_INTEGRATOR_RECURSIVE, "Recursive ray tracing")
	appendMenu(integrator, MF_STRING, ID_INTEGRATOR_PHOTON, "Photon mapping")
	appendMenu(render, MF_POPUP, integrator, "Integrator")
	photon, _, _ := pCreatePopupMenu.Call()
	appendMenu(photon, MF_STRING, ID_PHOTON_OFF, "Off")
	appendMenu(photon, MF_STRING, ID_PHOTON_LOW, "Low (512 photons)")
	appendMenu(photon, MF_STRING, ID_PHOTON_MED, "Medium (2k photons)")
	appendMenu(photon, MF_STRING, ID_PHOTON_HIGH, "High (8k photons)")
	appendMenu(photon, MF_STRING, ID_PHOTON_ULTRA, "Ultra (16k photons)")
	appendMenu(render, MF_POPUP, photon, "Photon mapping quality")
	upscale, _, _ := pCreatePopupMenu.Call()
	appendMenu(upscale, MF_STRING, ID_UPSCALE_OFF, "Off")
	appendMenu(upscale, MF_STRING, ID_UPSCALE_UQ, "Ultra Quality (77% internal)")
	appendMenu(upscale, MF_STRING, ID_UPSCALE_BALANCED, "Balanced (67% internal)")
	appendMenu(upscale, MF_STRING, ID_UPSCALE_PERF, "Performance (50% internal)")
	appendMenu(render, MF_POPUP, upscale, "DLSS Research Reconstruction")

	aa, _, _ := pCreatePopupMenu.Call()
	fxaa, _, _ := pCreatePopupMenu.Call()
	appendMenu(fxaa, MF_STRING, ID_FXAA_OFF, "Off")
	appendMenu(fxaa, MF_STRING, ID_FXAA_LOW, "Low")
	appendMenu(fxaa, MF_STRING, ID_FXAA_MED, "Medium")
	appendMenu(fxaa, MF_STRING, ID_FXAA_HIGH, "High")
	appendMenu(aa, MF_POPUP, fxaa, "FXAA")
	taa, _, _ := pCreatePopupMenu.Call()
	appendMenu(taa, MF_STRING, ID_TAA_OFF, "Off")
	appendMenu(taa, MF_STRING, ID_TAA_LOW, "Low")
	appendMenu(taa, MF_STRING, ID_TAA_MED, "Medium")
	appendMenu(taa, MF_STRING, ID_TAA_HIGH, "High")
	appendMenu(aa, MF_POPUP, taa, "TAA")
	tsaa, _, _ := pCreatePopupMenu.Call()
	appendMenu(tsaa, MF_STRING, ID_TSAA_OFF, "Off")
	appendMenu(tsaa, MF_STRING, ID_TSAA_LOW, "2x temporal supersample")
	appendMenu(tsaa, MF_STRING, ID_TSAA_MED, "3x temporal supersample")
	appendMenu(tsaa, MF_STRING, ID_TSAA_HIGH, "4x temporal supersample")
	appendMenu(aa, MF_POPUP, tsaa, "TSAA")
	txaa, _, _ := pCreatePopupMenu.Call()
	appendMenu(txaa, MF_STRING, ID_TXAA_OFF, "Off")
	appendMenu(txaa, MF_STRING, ID_TXAA_LOW, "Low")
	appendMenu(txaa, MF_STRING, ID_TXAA_MED, "Medium")
	appendMenu(txaa, MF_STRING, ID_TXAA_HIGH, "High")
	appendMenu(aa, MF_POPUP, txaa, "TXAA-style")
	msaa, _, _ := pCreatePopupMenu.Call()
	appendMenu(msaa, MF_STRING, ID_MSAA_1, "1x")
	appendMenu(msaa, MF_STRING, ID_MSAA_2, "2x")
	appendMenu(msaa, MF_STRING, ID_MSAA_4, "4x")
	appendMenu(msaa, MF_STRING, ID_MSAA_8, "8x")
	appendMenu(aa, MF_POPUP, msaa, "MSAA / stochastic SSAA")
	appendMenu(render, MF_POPUP, aa, "Anti-aliasing")

	post, _, _ := pCreatePopupMenu.Call()
	hbao, _, _ := pCreatePopupMenu.Call()
	appendMenu(hbao, MF_STRING, ID_HBAO_OFF, "Off")
	appendMenu(hbao, MF_STRING, ID_HBAO_LOW, "Low")
	appendMenu(hbao, MF_STRING, ID_HBAO_MED, "Medium")
	appendMenu(hbao, MF_STRING, ID_HBAO_HIGH, "High")
	appendMenu(post, MF_POPUP, hbao, "HBAO")
	hbaop, _, _ := pCreatePopupMenu.Call()
	appendMenu(hbaop, MF_STRING, ID_HBAOP_OFF, "Off")
	appendMenu(hbaop, MF_STRING, ID_HBAOP_LOW, "Low")
	appendMenu(hbaop, MF_STRING, ID_HBAOP_MED, "Medium")
	appendMenu(hbaop, MF_STRING, ID_HBAOP_HIGH, "High")
	appendMenu(post, MF_POPUP, hbaop, "HBAO+")
	bloom, _, _ := pCreatePopupMenu.Call()
	appendMenu(bloom, MF_STRING, ID_BLOOM_OFF, "Off")
	appendMenu(bloom, MF_STRING, ID_BLOOM_LOW, "Low")
	appendMenu(bloom, MF_STRING, ID_BLOOM_MED, "Medium")
	appendMenu(bloom, MF_STRING, ID_BLOOM_HIGH, "High")
	appendMenu(post, MF_POPUP, bloom, "Bloom")
	hdr, _, _ := pCreatePopupMenu.Call()
	appendMenu(hdr, MF_STRING, ID_HDR_OFF, "Off / clamp")
	appendMenu(hdr, MF_STRING, ID_HDR_REINHARD, "Reinhard")
	appendMenu(hdr, MF_STRING, ID_HDR_FILMIC, "Filmic")
	appendMenu(hdr, MF_STRING, ID_HDR_ACES, "ACES-style")
	appendMenu(post, MF_POPUP, hdr, "HDR tone map")
	exposure, _, _ := pCreatePopupMenu.Call()
	appendMenu(exposure, MF_STRING, ID_EXPOSURE_MINUS, "Exposure -1 EV")
	appendMenu(exposure, MF_STRING, ID_EXPOSURE_ZERO, "Exposure 0 EV")
	appendMenu(exposure, MF_STRING, ID_EXPOSURE_PLUS, "Exposure +1 EV")
	appendMenu(post, MF_POPUP, exposure, "Exposure")
	gamma, _, _ := pCreatePopupMenu.Call()
	appendMenu(gamma, MF_STRING, ID_GAMMA_20, "Gamma 2.0")
	appendMenu(gamma, MF_STRING, ID_GAMMA_22, "Gamma 2.2")
	appendMenu(gamma, MF_STRING, ID_GAMMA_24, "Gamma 2.4")
	appendMenu(post, MF_POPUP, gamma, "Gamma")
	appendMenu(render, MF_POPUP, post, "Lighting & post")

	present, _, _ := pCreatePopupMenu.Call()
	appendMenu(present, MF_STRING, ID_PRESENT_15, "15 Hz (quiet)")
	appendMenu(present, MF_STRING, ID_PRESENT_30, "30 Hz (balanced)")
	appendMenu(present, MF_STRING, ID_PRESENT_60, "60 Hz (smooth)")
	appendMenu(render, MF_POPUP, present, "Viewport update rate")
	perf, _, _ := pCreatePopupMenu.Call()
	tiles, _, _ := pCreatePopupMenu.Call()
	appendMenu(tiles, MF_STRING, ID_TILE_8, "8 x 8")
	appendMenu(tiles, MF_STRING, ID_TILE_16, "16 x 16")
	appendMenu(tiles, MF_STRING, ID_TILE_32, "32 x 32")
	appendMenu(perf, MF_POPUP, tiles, "Tile size")
	workers, _, _ := pCreatePopupMenu.Call()
	appendMenu(workers, MF_STRING, ID_WORKERS_AUTO, "Auto / all available")
	appendMenu(workers, MF_STRING, ID_WORKERS_1, "1 worker thread")
	appendMenu(workers, MF_STRING, ID_WORKERS_2, "2 worker threads")
	appendMenu(workers, MF_STRING, ID_WORKERS_4, "4 worker threads")
	appendMenu(workers, MF_STRING, ID_WORKERS_8, "8 worker threads")
	appendMenu(workers, MF_STRING, ID_WORKERS_16, "16 worker threads")
	appendMenu(workers, MF_STRING, ID_WORKERS_32, "32 worker threads")
	appendMenu(perf, MF_POPUP, workers, "CPU workers")
	appendMenu(perf, MF_STRING, ID_ADAPTIVE_SAMPLING, "Toggle adaptive sampling")
	adapt, _, _ := pCreatePopupMenu.Call()
	appendMenu(adapt, MF_STRING, ID_ADAPT_LOOSE, "Loose / interactive")
	appendMenu(adapt, MF_STRING, ID_ADAPT_MED, "Balanced")
	appendMenu(adapt, MF_STRING, ID_ADAPT_TIGHT, "Tight / final")
	appendMenu(perf, MF_POPUP, adapt, "Adaptive threshold")
	sampler, _, _ := pCreatePopupMenu.Call()
	appendMenu(sampler, MF_STRING, ID_SAMPLER_RANDOM, "Random")
	appendMenu(sampler, MF_STRING, ID_SAMPLER_HALTON, "Halton low discrepancy")
	appendMenu(sampler, MF_STRING, ID_SAMPLER_R2, "R2 quasi-random")
	appendMenu(perf, MF_POPUP, sampler, "Sampling sequence")
	transport, _, _ := pCreatePopupMenu.Call()
	appendMenu(transport, MF_STRING, ID_MIS_TOGGLE, "Toggle power-heuristic MIS")
	appendMenu(transport, MF_STRING, ID_POWER_LIGHT_TOGGLE, "Toggle power-weighted light selection")
	clampMenu, _, _ := pCreatePopupMenu.Call()
	appendMenu(clampMenu, MF_STRING, ID_CLAMP_OFF, "Off")
	appendMenu(clampMenu, MF_STRING, ID_CLAMP_12, "12")
	appendMenu(clampMenu, MF_STRING, ID_CLAMP_24, "24")
	appendMenu(clampMenu, MF_STRING, ID_CLAMP_48, "48")
	appendMenu(transport, MF_POPUP, clampMenu, "Firefly clamp")
	rr, _, _ := pCreatePopupMenu.Call()
	appendMenu(rr, MF_STRING, ID_RR_2, "Start at bounce 2")
	appendMenu(rr, MF_STRING, ID_RR_4, "Start at bounce 4")
	appendMenu(rr, MF_STRING, ID_RR_6, "Start at bounce 6")
	appendMenu(transport, MF_POPUP, rr, "Russian roulette")
	appendMenu(perf, MF_POPUP, transport, "Light transport research")
	accel, _, _ := pCreatePopupMenu.Call()
	bvh, _, _ := pCreatePopupMenu.Call()
	appendMenu(bvh, MF_STRING, ID_BVH_MEDIAN, "Median BVH")
	appendMenu(bvh, MF_STRING, ID_BVH_SAH8, "Binned SAH - 8 bins")
	appendMenu(bvh, MF_STRING, ID_BVH_SAH16, "Binned SAH - 16 bins")
	appendMenu(bvh, MF_STRING, ID_BVH_SAH32, "Binned SAH - 32 bins")
	appendMenu(accel, MF_POPUP, bvh, "BVH builder")
	leaf, _, _ := pCreatePopupMenu.Call()
	appendMenu(leaf, MF_STRING, ID_LEAF_2, "2 primitives / leaf")
	appendMenu(leaf, MF_STRING, ID_LEAF_4, "4 primitives / leaf")
	appendMenu(leaf, MF_STRING, ID_LEAF_8, "8 primitives / leaf")
	appendMenu(accel, MF_POPUP, leaf, "BVH leaf size")
	tileOrder, _, _ := pCreatePopupMenu.Call()
	appendMenu(tileOrder, MF_STRING, ID_TILE_CENTER, "Center-first")
	appendMenu(tileOrder, MF_STRING, ID_TILE_SCANLINE, "Scanline")
	appendMenu(accel, MF_POPUP, tileOrder, "Tile ordering")
	publish, _, _ := pCreatePopupMenu.Call()
	appendMenu(publish, MF_STRING, ID_PUBLISH_15, "15 update batches")
	appendMenu(publish, MF_STRING, ID_PUBLISH_30, "30 update batches")
	appendMenu(publish, MF_STRING, ID_PUBLISH_60, "60 update batches")
	appendMenu(accel, MF_POPUP, publish, "Progressive publish rate")
	appendMenu(accel, MF_STRING, ID_PROGRESSIVE_TOGGLE, "Toggle progressive framebuffer updates")
	appendMenu(perf, MF_POPUP, accel, "Acceleration / scheduling")
	hyper, _, _ := pCreatePopupMenu.Call()
	appendMenu(hyper, MF_STRING, ID_HYPER_LATENCY, "Latency / interaction")
	appendMenu(hyper, MF_STRING, ID_HYPER_THROUGHPUT, "Maximum throughput")
	appendMenu(hyper, MF_STRING, ID_HYPER_FINAL, "Final-quality efficiency")
	appendMenu(hyper, MF_STRING, ID_HYPER_MULTICORE, "Multicore saturation")
	appendMenu(perf, MF_POPUP, hyper, "Hyper-optimisation presets")
	appendMenu(render, MF_POPUP, perf, "Performance / optimisation")
	appendMenu(render, MF_SEPARATOR, 0, "")
	appendMenu(render, MF_STRING, ID_TOGGLE_DENOISE, "Toggle guide-aware denoise")
	appendMenu(render, MF_STRING, ID_TOGGLE_AUTORENDER, "Toggle auto-render after movement")
	appendMenu(render, MF_STRING, ID_LIVE_PREVIEW, "Toggle realtime movement preview")
	appendMenu(bar, MF_POPUP, render, "Render")
	camera, _, _ := pCreatePopupMenu.Call()
	appendMenu(camera, MF_STRING, ID_CAMERA_RESET, "Reset camera")
	appendMenu(camera, MF_SEPARATOR, 0, "")
	appendMenu(camera, MF_STRING, ID_CAMERA_LEFT, "Orbit left")
	appendMenu(camera, MF_STRING, ID_CAMERA_RIGHT, "Orbit right")
	appendMenu(camera, MF_STRING, ID_CAMERA_UP, "Orbit up")
	appendMenu(camera, MF_STRING, ID_CAMERA_DOWN, "Orbit down")
	appendMenu(camera, MF_STRING, ID_CAMERA_NEAR, "Move closer")
	appendMenu(camera, MF_STRING, ID_CAMERA_FAR, "Move farther")
	appendMenu(camera, MF_SEPARATOR, 0, "")
	speed, _, _ := pCreatePopupMenu.Call()
	appendMenu(speed, MF_STRING, ID_SPEED_FINE, "Fine")
	appendMenu(speed, MF_STRING, ID_SPEED_NORMAL, "Normal")
	appendMenu(speed, MF_STRING, ID_SPEED_FAST, "Fast")
	appendMenu(camera, MF_POPUP, speed, "Navigation speed")
	appendMenu(bar, MF_POPUP, camera, "Camera")
	objects, _, _ := pCreatePopupMenu.Call()
	appendMenu(objects, MF_STRING, ID_OBJECT_EDIT, "Toggle object edit / drag mode")
	appendMenu(objects, MF_SEPARATOR, 0, "")
	appendMenu(objects, MF_STRING, ID_OBJECT_DIFFUSE, "Add diffuse sphere")
	appendMenu(objects, MF_STRING, ID_OBJECT_METAL, "Add metal sphere")
	appendMenu(objects, MF_STRING, ID_OBJECT_GLASS, "Add glass sphere")
	appendMenu(objects, MF_STRING, ID_OBJECT_LIGHT, "Add emissive light sphere")
	appendMenu(objects, MF_STRING, ID_OBJECT_CUBE, "Add cube")
	appendMenu(bar, MF_POPUP, objects, "Objects")
	view, _, _ := pCreatePopupMenu.Call()
	appendMenu(view, MF_STRING, ID_VIEW_LEFT, "Rotate final render left\t[")
	appendMenu(view, MF_STRING, ID_VIEW_RIGHT, "Rotate final render right\t]")
	appendMenu(view, MF_STRING, ID_VIEW_RESET, "Reset render rotation")
	appendMenu(bar, MF_POPUP, view, "View")
	debug, _, _ := pCreatePopupMenu.Call()
	appendMenu(debug, MF_STRING, ID_DEBUG_BEAUTY, "Beauty / final image")
	appendMenu(debug, MF_STRING, ID_DEBUG_ALBEDO, "Albedo")
	appendMenu(debug, MF_STRING, ID_DEBUG_NORMAL, "Normals")
	appendMenu(debug, MF_STRING, ID_DEBUG_DEPTH, "Depth")
	appendMenu(debug, MF_STRING, ID_DEBUG_AO, "Ambient occlusion")
	appendMenu(debug, MF_STRING, ID_DEBUG_HEAT, "Luminance heatmap")
	appendMenu(bar, MF_POPUP, debug, "Debug")
	bench, _, _ := pCreatePopupMenu.Call()
	appendMenu(bench, MF_STRING, ID_BENCH_QUICK, "Quick CPU benchmark")
	appendMenu(bench, MF_STRING, ID_BENCH_FULL, "FULL GIGA benchmark")
	appendMenu(bench, MF_SEPARATOR, 0, "")
	appendMenu(bench, MF_STRING, ID_BENCH_SCALING, "Multicore scaling")
	appendMenu(bench, MF_STRING, ID_BENCH_POST, "Post-processing / reconstruction")
	appendMenu(bench, MF_STRING, ID_BENCH_INTEGRATORS, "Integrator comparison")
	appendMenu(bench, MF_STRING, ID_BENCH_GPU, "GPU / OpenCL probe")
	appendMenu(bench, MF_SEPARATOR, 0, "")
	appendMenu(bench, MF_STRING, ID_BENCH_SHOW_LAST, "Show last benchmark report")
	appendMenu(bar, MF_POPUP, bench, "Benchmark")
	help, _, _ := pCreatePopupMenu.Call()
	appendMenu(help, MF_STRING, ID_HELP_ABOUT, "About Beamcast Studio")
	appendMenu(bar, MF_POPUP, help, "Help")
	return bar
}

func openDialog(hwnd uintptr) (string, bool) {
	file := make([]uint16, 4096)
	filter := multiW("Wavefront OBJ (*.obj)", "*.obj", "All files (*.*)", "*.*")
	title := wchars("Open Wavefront OBJ")
	of := OPENFILENAME{LStructSize: uint32(unsafe.Sizeof(OPENFILENAME{})), HwndOwner: hwnd, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &file[0], NMaxFile: uint32(len(file)), LpstrTitle: &title[0], Flags: OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST | OFN_HIDEREADONLY}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return "", false
	}
	n := 0
	for n < len(file) && file[n] != 0 {
		n++
	}
	return string(utf16.Decode(file[:n])), true
}
func saveDialog(hwnd uintptr, ext, desc string) (string, bool) {
	file := make([]uint16, 4096)
	initial := wchars("beamcast_render." + ext)
	copy(file, initial)
	filter := multiW(desc, "*."+ext, "All files (*.*)", "*.*")
	title := wchars("Save Beamcast Render")
	def := wchars(ext)
	of := OPENFILENAME{LStructSize: uint32(unsafe.Sizeof(OPENFILENAME{})), HwndOwner: hwnd, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &file[0], NMaxFile: uint32(len(file)), LpstrTitle: &title[0], Flags: OFN_OVERWRITEPROMPT | OFN_PATHMUSTEXIST, LpstrDefExt: &def[0]}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return "", false
	}
	n := 0
	for n < len(file) && file[n] != 0 {
		n++
	}
	return string(utf16.Decode(file[:n])), true
}

func message(hwnd uintptr, title, msg string, flags uintptr) {
	pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(ptr(msg))), uintptr(unsafe.Pointer(ptr(title))), flags)
}

type Studio struct {
	hwnd             uintptr
	scene            *Scene
	camera           Camera
	defaultCamera    Camera
	settings         RenderSettings
	render           *RenderState
	rotation         int
	autoRender       bool
	clientW, clientH int
	dragging         bool
	dragButton       int
	lastX, lastY     int
	uiStatus         string
	lastFrame        time.Time
	cachedSerial     uint64
	presentHz        int
	orbitScale       float64
	panScale         float64
	dollyScale       float64
	cameraSpeed      string
	livePreview      bool
	lastPreviewStart time.Time
	objectEdit       bool
	selectedObject   int
	cachedRotation   int
	cachedImageW     int
	cachedImageH     int
	cachedWinPix     []uint32
	lastTimerRunning bool
	benchmarkMu      sync.RWMutex
	benchmarkRunning bool
	lastBenchmark    string
}

var app *Studio

func NewStudio() *Studio {
	s, c := NewShowcaseScene()
	return &Studio{scene: s, camera: c, defaultCamera: c, settings: Quality("Preview"), render: NewRenderState(), autoRender: true, clientW: 1600, clientH: 980, uiStatus: "Ready", presentHz: 30, orbitScale: 0.28, panScale: 0.0015, dollyScale: 0.6, cameraSpeed: "Normal", livePreview: true, selectedObject: -1}
}
func (s *Studio) cancelAndWait() {
	s.render.CancelRender()
	for s.render.Meta().Rendering {
		time.Sleep(3 * time.Millisecond)
	}
}
func (s *Studio) setScene(scene *Scene, cam Camera) {
	s.cancelAndWait()
	s.scene = scene
	s.camera = cam
	s.defaultCamera = cam
	s.render = NewRenderState()
	s.rotation = 0
	s.cachedSerial = 0
	s.cachedWinPix = nil
	s.uiStatus = "Scene ready: " + scene.Name
	s.invalidate()
}
func (s *Studio) configureSceneAcceleration() bool {
	if s.scene == nil {
		return false
	}
	dirty := len(s.scene.Nodes) == 0 ||
		!strings.EqualFold(s.scene.BVHMode, s.settings.BVHMode) ||
		s.scene.BVHBins != s.settings.BVHBins ||
		s.scene.BVHLeafSize != s.settings.BVHLeafSize
	s.scene.BVHMode = s.settings.BVHMode
	s.scene.BVHBins = s.settings.BVHBins
	s.scene.BVHLeafSize = s.settings.BVHLeafSize
	return dirty
}

func (s *Studio) startRender() {
	if s.scene == nil {
		return
	}
	s.cancelAndWait()
	if s.configureSceneAcceleration() {
		s.scene.Build()
	}
	s.render.Start(s.scene, s.camera, s.settings)
	s.uiStatus = "Rendering " + s.settings.Quality + " | " + string(s.settings.Integrator)
	s.invalidate()
}
func (s *Studio) maybeAutoRender() {
	if s.autoRender {
		s.startRender()
	} else {
		s.invalidate()
	}
}
func (s *Studio) invalidate() {
	if s.hwnd != 0 {
		pInvalidateRect.Call(s.hwnd, 0, 0)
	}
}
func (s *Studio) viewportRect() RECT {
	panel := int32(460)
	return RECT{0, 0, int32(maxInt(1, s.clientW-int(panel))), int32(maxInt(1, s.clientH-28))}
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func (s *Studio) panelX() int { return maxInt(0, s.clientW-460) }

func (s *Studio) panelRect(id int) RECT {
	px := s.panelX()
	left := int32(px + 22)
	right := int32(s.clientW - 22)
	gap := int32(10)
	mid := left + (right-left-gap)/2
	full := func(top, bottom int32) RECT { return RECT{left, top, right, bottom} }
	colL := func(top, bottom int32) RECT { return RECT{left, top, mid, bottom} }
	colR := func(top, bottom int32) RECT { return RECT{mid + gap, top, right, bottom} }
	switch id {
	case ID_RENDER_START:
		return full(56, 90)
	case ID_RENDER_CANCEL:
		return full(98, 126)
	case ID_PANEL_QUALITY:
		return full(164, 194)
	case ID_TOGGLE_DENOISE:
		return colL(202, 232)
	case ID_TOGGLE_AUTORENDER:
		return colR(202, 232)
	case ID_LIVE_PREVIEW:
		return full(240, 270)
	case ID_CAMERA_LEFT:
		return colL(408, 438)
	case ID_CAMERA_RIGHT:
		return colR(408, 438)
	case ID_CAMERA_UP:
		return colL(446, 476)
	case ID_CAMERA_DOWN:
		return colR(446, 476)
	case ID_CAMERA_NEAR:
		return colL(484, 514)
	case ID_CAMERA_FAR:
		return colR(484, 514)
	case ID_CAMERA_RESET:
		return full(522, 552)
	case ID_OBJECT_EDIT:
		return full(588, 618)
	case ID_VIEW_LEFT:
		return colL(688, 718)
	case ID_VIEW_RIGHT:
		return colR(688, 718)
	case ID_VIEW_RESET:
		return full(726, 756)
	default:
		return RECT{}
	}
}
func (s *Studio) cycleQuality() {
	names := []string{"Draft", "Preview", "Balanced", "High", "Ultra", "Ultra Realism"}
	idx := 0
	for i, n := range names {
		if s.settings.Quality == n {
			idx = i
		}
	}
	nextName := names[(idx+1)%len(names)]
	if nextName == "Ultra Realism" {
		s.applyUltraRealism()
		return
	}
	next := Quality(nextName)
	next.Backend = s.settings.Backend
	next.Upscale = s.settings.Upscale
	next.Integrator = s.settings.Integrator
	next.PhotonMapping = s.settings.PhotonMapping
	next.DebugView = s.settings.DebugView
	next.TileSize = s.settings.TileSize
	next.CPUWorkers = s.settings.CPUWorkers
	next.AdaptiveSampling = s.settings.AdaptiveSampling
	next.AdaptiveThreshold = s.settings.AdaptiveThreshold
	next.Sampler = s.settings.Sampler
	next.MIS = s.settings.MIS
	next.PowerLightSampling = s.settings.PowerLightSampling
	next.FireflyClamp = s.settings.FireflyClamp
	next.RRDepth = s.settings.RRDepth
	next.TileOrder = s.settings.TileOrder
	next.PublishHz = s.settings.PublishHz
	next.ProgressiveUpdates = s.settings.ProgressiveUpdates
	next.BVHMode = s.settings.BVHMode
	next.BVHBins = s.settings.BVHBins
	next.BVHLeafSize = s.settings.BVHLeafSize
	next.Exposure = s.settings.Exposure
	next.Gamma = s.settings.Gamma
	s.settings = next
	s.uiStatus = "Quality: " + s.settings.Quality
	s.invalidate()
}

func (s *Studio) applyUltraRealism() {
	backend := s.settings.Backend
	exposure := s.settings.Exposure
	gamma := s.settings.Gamma
	next := UltraRealismSettings(backend)
	next.Exposure = exposure
	next.Gamma = gamma
	s.settings = next
	if backend == BackendGPU {
		s.uiStatus = "Ultra Realism: GPU-compatible high-sample path tracing"
	} else {
		s.uiStatus = "Ultra Realism: MIS + Halton + deep native path tracing"
	}
	s.invalidate()
}

func (s *Studio) setResolution(w, h int) {
	s.settings.Width = w
	s.settings.Height = h
	s.uiStatus = fmt.Sprintf("Resolution: %d x %d", w, h)
	s.invalidate()
}

func (s *Studio) resolutionLabel() string {
	return fmt.Sprintf("%d x %d", s.settings.Width, s.settings.Height)
}
func (s *Studio) applyQuality(name string) {
	next := Quality(name)
	next.Backend = s.settings.Backend
	next.Upscale = s.settings.Upscale
	next.Integrator = s.settings.Integrator
	next.PhotonMapping = s.settings.PhotonMapping
	next.DebugView = s.settings.DebugView
	next.TileSize = s.settings.TileSize
	next.CPUWorkers = s.settings.CPUWorkers
	next.AdaptiveSampling = s.settings.AdaptiveSampling
	next.AdaptiveThreshold = s.settings.AdaptiveThreshold
	next.Sampler = s.settings.Sampler
	next.MIS = s.settings.MIS
	next.PowerLightSampling = s.settings.PowerLightSampling
	next.FireflyClamp = s.settings.FireflyClamp
	next.RRDepth = s.settings.RRDepth
	next.Denoise = s.settings.Denoise
	next.Exposure = s.settings.Exposure
	next.Gamma = s.settings.Gamma
	s.settings = next
	s.uiStatus = "Quality: " + s.settings.Quality
	s.invalidate()
}

func (s *Studio) setExposure(v float64) {
	s.settings.Exposure = v
	s.uiStatus = fmt.Sprintf("Exposure: %.1f EV", v)
	s.invalidate()
}

func (s *Studio) setGamma(v float64) {
	s.settings.Gamma = v
	s.uiStatus = fmt.Sprintf("Gamma: %.1f", v)
	s.invalidate()
}

func (s *Studio) setPresentHz(hz int) {
	if hz < 1 {
		hz = 30
	}
	s.presentHz = hz
	if s.hwnd != 0 {
		pKillTimer.Call(s.hwnd, 1)
		pSetTimer.Call(s.hwnd, 1, uintptr(1000/hz), 0)
	}
	s.uiStatus = fmt.Sprintf("Viewport update rate: %d Hz", hz)
	s.invalidate()
}

func (s *Studio) setCameraSpeed(mode string) {
	s.cameraSpeed = mode
	switch mode {
	case "Fine":
		s.orbitScale = 0.17
		s.panScale = 0.0009
		s.dollyScale = 0.35
	case "Fast":
		s.orbitScale = 0.45
		s.panScale = 0.0024
		s.dollyScale = 1.0
	default:
		s.cameraSpeed = "Normal"
		s.orbitScale = 0.28
		s.panScale = 0.0015
		s.dollyScale = 0.6
	}
	s.uiStatus = "Camera speed: " + s.cameraSpeed
	s.invalidate()
}

func (s *Studio) toneLabel() string {
	return fmt.Sprintf("Exp %.1f EV | Gamma %.1f", s.settings.Exposure, s.settings.Gamma)
}

func (s *Studio) smoothnessLabel() string {
	return fmt.Sprintf("%d Hz | %s nav", s.presentHz, s.cameraSpeed)
}

func (s *Studio) rotate(delta int) {
	s.rotation = (s.rotation + delta) % 4
	if s.rotation < 0 {
		s.rotation += 4
	}
	s.uiStatus = fmt.Sprintf("Displayed render rotated %d°", s.rotation*90)
	s.invalidate()
}

func (s *Studio) backendLabel() string    { return string(s.settings.Backend) }
func (s *Studio) upscaleLabel() string    { return string(s.settings.Upscale) }
func (s *Studio) integratorLabel() string { return string(s.settings.Integrator) }
func (s *Studio) photonLabel() string {
	switch s.settings.PhotonMapping {
	case 1:
		return "Low"
	case 2:
		return "Med"
	case 3:
		return "High"
	case 4:
		return "Ultra"
	default:
		return "Off"
	}
}
func (s *Studio) debugLabel() string {
	switch s.settings.DebugView {
	case DebugAlbedo:
		return "Albedo"
	case DebugNormal:
		return "Normals"
	case DebugDepth:
		return "Depth"
	case DebugAO:
		return "AO"
	case DebugHeat:
		return "Heat"
	default:
		return "Beauty"
	}
}
func (s *Studio) perfLabel() string {
	workers := "Auto"
	if s.settings.CPUWorkers > 0 {
		workers = fmt.Sprintf("%d", s.settings.CPUWorkers)
	}
	return fmt.Sprintf("tile %d | workers %s | adapt %v | %s", s.settings.TileSize, workers, s.settings.AdaptiveSampling, s.settings.Sampler)
}

func (s *Studio) transportLabel() string {
	clampLabel := "off"
	if s.settings.FireflyClamp > 0 {
		clampLabel = fmt.Sprintf("%.0f", s.settings.FireflyClamp)
	}
	return fmt.Sprintf("MIS %v | power-light %v | clamp %s | RR %d", s.settings.MIS, s.settings.PowerLightSampling, clampLabel, s.settings.RRDepth)
}

func (s *Studio) editCamera(fn func(*Camera)) {
	fn(&s.camera)
	s.uiStatus = "Camera changed"
	s.maybeAutoRender()
}

func levelName(v int) string {
	switch v {
	case 1:
		return "Low"
	case 2:
		return "Med"
	case 3:
		return "High"
	default:
		return "Off"
	}
}

func (s *Studio) aaSummary() string {
	return fmt.Sprintf("FXAA %s | TAA %s | TSAA %s | TXAA %s | MSAA %dx", levelName(s.settings.FXAA), levelName(s.settings.TAA), levelName(s.settings.TSAA), levelName(s.settings.TXAA), s.settings.MSAA)
}

func (s *Studio) postSummary() string {
	ao := "HBAO " + levelName(s.settings.HBAO)
	if s.settings.HBAOPlus > 0 {
		ao = "HBAO+ " + levelName(s.settings.HBAOPlus)
	}
	return fmt.Sprintf("%s | Bloom %s | HDR %d", ao, levelName(s.settings.Bloom), s.settings.HDR)
}

func (s *Studio) startPreviewRender() {
	if !s.livePreview || s.scene == nil {
		return
	}
	if time.Since(s.lastPreviewStart) < 140*time.Millisecond {
		return
	}
	s.lastPreviewStart = time.Now()
	q := s.settings
	aspect := float64(maxInt(q.Width, 1)) / float64(maxInt(q.Height, 1))
	q.Width = 520
	q.Height = maxInt(240, int(float64(q.Width)/math.Max(aspect, 0.2)))
	q.SPP = 1
	q.Bounces = minInt(q.Bounces, 3)
	q.Denoise = false
	q.Upscale = UpscaleOff
	q.FXAA = 1
	q.TAA = 0
	q.TSAA = 0
	q.TXAA = 0
	q.HBAO = 0
	q.HBAOPlus = 0
	q.Bloom = 0
	q.MSAA = 1
	q.TileSize = 32
	q.TileOrder = "Center"
	q.PublishHz = 60
	q.ProgressiveUpdates = true
	q.AdaptiveSampling = true
	if q.Integrator == IntegratorPhoton && q.PhotonMapping < 1 {
		q.PhotonMapping = 1
	}
	s.render.Start(s.scene, s.camera, q)
	s.uiStatus = "Realtime preview"
}

func (s *Studio) viewportRay(x, y int) Ray {
	vp := s.viewportRect()
	w := maxInt(2, int(vp.Right-vp.Left))
	h := maxInt(2, int(vp.Bottom-vp.Top))
	u := clamp(float64(x-int(vp.Left))/float64(w-1), 0, 1)
	v := clamp(float64(h-1-(y-int(vp.Top)))/float64(h-1), 0, 1)
	origin, lower, horiz, vert := s.camera.basis(float64(w) / float64(h))
	return Ray{origin, lower.Add(horiz.Mul(u)).Add(vert.Mul(v)).Sub(origin)}
}

func (s *Studio) addPresetObject(kind string) {
	if s.scene == nil {
		return
	}
	s.cancelAndWait()
	forward := s.camera.Target.Sub(s.camera.Position).Unit()
	pos := s.camera.Target.Add(forward.Mul(0.25))
	idx := s.scene.AddPresetObject(kind, pos)
	if idx >= 0 {
		s.selectedObject = idx
		s.objectEdit = true
		s.uiStatus = "Added " + kind + " - object edit mode on"
		s.maybeAutoRender()
	}
}

func (s *Studio) selectedObjectLabel() string {
	if s.scene == nil || s.selectedObject < 0 || s.selectedObject >= len(s.scene.Objects) {
		return "Selected: none"
	}
	return "Selected: " + s.scene.Objects[s.selectedObject].Name
}

func (s *Studio) launchBenchmark(kind, title string) {
	s.benchmarkMu.Lock()
	if s.benchmarkRunning {
		s.benchmarkMu.Unlock()
		message(s.hwnd, "Beamcast GIGA Benchmark", "A benchmark is already running.", MB_OK|MB_ICONINFORMATION)
		return
	}
	s.benchmarkRunning = true
	s.benchmarkMu.Unlock()
	s.cancelAndWait()
	s.uiStatus = "GIGA benchmark running: " + title
	s.invalidate()
	hwnd := s.hwnd
	go func() {
		report := RunGigaBenchmark(kind)
		s.benchmarkMu.Lock()
		s.lastBenchmark = report
		s.benchmarkRunning = false
		s.benchmarkMu.Unlock()
		pPostMessageW.Call(hwnd, WM_BENCH_DONE, 0, 0)
	}()
}

func (s *Studio) benchmarkReport() (string, bool) {
	s.benchmarkMu.RLock()
	defer s.benchmarkMu.RUnlock()
	return s.lastBenchmark, s.benchmarkRunning
}

func (s *Studio) showLastBenchmark() {
	report, running := s.benchmarkReport()
	if report == "" {
		if running {
			message(s.hwnd, "Beamcast GIGA Benchmark", "Benchmark is still running.", MB_OK|MB_ICONINFORMATION)
		} else {
			message(s.hwnd, "Beamcast GIGA Benchmark", "No benchmark report has been generated yet.", MB_OK|MB_ICONINFORMATION)
		}
		return
	}
	message(s.hwnd, "Beamcast GIGA Benchmark Results", report, MB_OK|MB_ICONINFORMATION)
}

func (s *Studio) command(id int) {
	switch id {
	case ID_FILE_NEW, ID_SCENE_SHOWCASE:
		a, c := NewShowcaseScene()
		s.setScene(a, c)
	case ID_SCENE_CORNELL:
		a, c := NewCornellScene()
		s.setScene(a, c)
	case ID_SCENE_STRESS:
		a, c := NewStressScene()
		s.setScene(a, c)
	case ID_FILE_OPEN:
		if p, ok := openDialog(s.hwnd); ok {
			sc, c, err := LoadOBJ(p)
			if err != nil {
				message(s.hwnd, "OBJ import failed", err.Error(), MB_OK|MB_ICONERROR)
			} else {
				s.setScene(sc, c)
				s.uiStatus = "Loaded " + filepath.Base(p)
			}
		}
	case ID_FILE_SAVE_PPM:
		if p, ok := saveDialog(s.hwnd, "ppm", "PPM image (*.ppm)"); ok {
			w, h, pix := s.render.PixelsCopy()
			if len(pix) == 0 {
				message(s.hwnd, "Nothing to save", "Render an image first.", MB_OK|MB_ICONINFORMATION)
			} else if err := SavePPM(p, pix, w, h, s.rotation); err != nil {
				message(s.hwnd, "Save failed", err.Error(), MB_OK|MB_ICONERROR)
			} else {
				s.uiStatus = "Saved " + filepath.Base(p)
			}
		}
	case ID_FILE_SAVE_BMP:
		if p, ok := saveDialog(s.hwnd, "bmp", "Bitmap image (*.bmp)"); ok {
			w, h, pix := s.render.PixelsCopy()
			if len(pix) == 0 {
				message(s.hwnd, "Nothing to save", "Render an image first.", MB_OK|MB_ICONINFORMATION)
			} else if err := SaveBMP(p, pix, w, h, s.rotation); err != nil {
				message(s.hwnd, "Save failed", err.Error(), MB_OK|MB_ICONERROR)
			} else {
				s.uiStatus = "Saved " + filepath.Base(p)
			}
		}
	case ID_FILE_SAVE_PFM:
		if p, ok := saveDialog(s.hwnd, "pfm", "Portable Float Map HDR (*.pfm)"); ok {
			w, h, linear := s.render.LinearCopy()
			if len(linear) == 0 {
				message(s.hwnd, "Nothing to save", "Render an image first.", MB_OK|MB_ICONINFORMATION)
			} else if err := SavePFM(p, linear, w, h, s.rotation); err != nil {
				message(s.hwnd, "HDR save failed", err.Error(), MB_OK|MB_ICONERROR)
			} else {
				s.uiStatus = "Saved HDR " + filepath.Base(p)
			}
		}
	case ID_FILE_EXIT:
		pDestroyWindow.Call(s.hwnd)
	case ID_RENDER_START:
		s.startRender()
	case ID_RENDER_CANCEL:
		s.render.CancelRender()
		s.uiStatus = "Cancelling render..."
		s.invalidate()
	case ID_QUALITY_DRAFT:
		s.applyQuality("Draft")
	case ID_QUALITY_PREVIEW:
		s.applyQuality("Preview")
	case ID_QUALITY_BALANCED:
		s.applyQuality("Balanced")
	case ID_QUALITY_HIGH:
		s.applyQuality("High")
	case ID_QUALITY_ULTRA:
		s.applyQuality("Ultra")
	case ID_QUALITY_REALISM:
		s.applyUltraRealism()
	case ID_PANEL_QUALITY:
		s.cycleQuality()
	case ID_TOGGLE_DENOISE:
		s.settings.Denoise = !s.settings.Denoise
		s.invalidate()
	case ID_TOGGLE_AUTORENDER:
		s.autoRender = !s.autoRender
		s.invalidate()
	case ID_LIVE_PREVIEW:
		s.livePreview = !s.livePreview
		s.uiStatus = fmt.Sprintf("Realtime movement preview: %v", s.livePreview)
		s.invalidate()
	case ID_RESOLUTION_540P:
		s.setResolution(640, 360)
	case ID_RESOLUTION_720P:
		s.setResolution(1280, 720)
	case ID_RESOLUTION_900P:
		s.setResolution(1600, 900)
	case ID_RESOLUTION_1080P:
		s.setResolution(1920, 1080)
	case ID_RESOLUTION_1440P:
		s.setResolution(2560, 1440)
	case ID_BACKEND_AUTO:
		s.settings.Backend = BackendAuto
		s.uiStatus = "Backend: Auto"
		s.invalidate()
	case ID_BACKEND_CPU:
		s.settings.Backend = BackendCPU
		s.uiStatus = "Backend: CPU"
		s.invalidate()
	case ID_BACKEND_GPU:
		s.settings.Backend = BackendGPU
		s.uiStatus = "Backend: OpenCL GPU (experimental)"
		s.invalidate()
	case ID_GPU_COMPAT:
		s.settings.Backend = BackendGPU
		s.settings.Integrator = IntegratorPath
		s.settings.Sampler = SamplerRandom
		s.settings.MIS = false
		s.settings.PowerLightSampling = false
		s.settings.FireflyClamp = 0
		s.settings.AdaptiveSampling = false
		s.uiStatus = "Applied OpenCL GPU compatibility preset"
		s.invalidate()
	case ID_GPU_DATAFLOW:
		s.settings.Backend = BackendGPU
		s.settings.Integrator = IntegratorPath
		s.settings.Sampler = SamplerRandom
		s.settings.MIS = false
		s.settings.PowerLightSampling = false
		s.settings.FireflyClamp = 0
		s.settings.AdaptiveSampling = false
		s.settings.ProgressiveUpdates = false
		s.settings.Denoise = false
		s.settings.HBAO = 0
		s.settings.HBAOPlus = 0
		s.settings.TAA = 0
		s.settings.TSAA = 0
		s.settings.TXAA = 0
		s.settings.FXAA = 0
		s.settings.Bloom = 0
		s.settings.DebugView = DebugBeauty
		s.uiStatus = "Applied GPU throughput / no-guide preset"
		s.invalidate()
	case ID_INTEGRATOR_PATH:
		s.settings.Integrator = IntegratorPath
		s.uiStatus = "Integrator: Path tracing"
		s.invalidate()
	case ID_INTEGRATOR_RECURSIVE:
		s.settings.Integrator = IntegratorRecursive
		s.uiStatus = "Integrator: Recursive ray tracing"
		s.invalidate()
	case ID_INTEGRATOR_PHOTON:
		s.settings.Integrator = IntegratorPhoton
		if s.settings.PhotonMapping == 0 {
			s.settings.PhotonMapping = 2
		}
		s.uiStatus = "Integrator: Photon mapping"
		s.invalidate()
	case ID_PHOTON_OFF:
		s.settings.PhotonMapping = 0
		s.uiStatus = "Photon mapping quality: Off"
		s.invalidate()
	case ID_PHOTON_LOW:
		s.settings.PhotonMapping = 1
		s.uiStatus = "Photon mapping quality: Low"
		s.invalidate()
	case ID_PHOTON_MED:
		s.settings.PhotonMapping = 2
		s.uiStatus = "Photon mapping quality: Medium"
		s.invalidate()
	case ID_PHOTON_HIGH:
		s.settings.PhotonMapping = 3
		s.uiStatus = "Photon mapping quality: High"
		s.invalidate()
	case ID_PHOTON_ULTRA:
		s.settings.PhotonMapping = 4
		s.uiStatus = "Photon mapping quality: Ultra"
		s.invalidate()
	case ID_UPSCALE_OFF:
		s.settings.Upscale = UpscaleOff
		s.uiStatus = "DLSS-style upscale: Off"
		s.invalidate()
	case ID_UPSCALE_UQ:
		s.settings.Upscale = UpscaleUltraQuality
		s.uiStatus = "DLSS-style upscale: Ultra Quality"
		s.invalidate()
	case ID_UPSCALE_BALANCED:
		s.settings.Upscale = UpscaleBalanced
		s.uiStatus = "DLSS-style upscale: Balanced"
		s.invalidate()
	case ID_UPSCALE_PERF:
		s.settings.Upscale = UpscalePerformance
		s.uiStatus = "DLSS-style upscale: Performance"
		s.invalidate()
	case ID_EXPOSURE_MINUS:
		s.setExposure(-1.0)
	case ID_EXPOSURE_ZERO:
		s.setExposure(0.0)
	case ID_EXPOSURE_PLUS:
		s.setExposure(1.0)
	case ID_GAMMA_20:
		s.setGamma(2.0)
	case ID_GAMMA_22:
		s.setGamma(2.2)
	case ID_GAMMA_24:
		s.setGamma(2.4)
	case ID_PRESENT_15:
		s.setPresentHz(15)
	case ID_PRESENT_30:
		s.setPresentHz(30)
	case ID_PRESENT_60:
		s.setPresentHz(60)
	case ID_TILE_8:
		s.settings.TileSize = 8
		s.uiStatus = "Tile size: 8"
		s.invalidate()
	case ID_TILE_16:
		s.settings.TileSize = 16
		s.uiStatus = "Tile size: 16"
		s.invalidate()
	case ID_TILE_32:
		s.settings.TileSize = 32
		s.uiStatus = "Tile size: 32"
		s.invalidate()
	case ID_WORKERS_AUTO:
		s.settings.CPUWorkers = 0
		s.uiStatus = "CPU workers: Auto / all available"
		s.invalidate()
	case ID_WORKERS_1:
		s.settings.CPUWorkers = 1
		s.uiStatus = "CPU workers: 1"
		s.invalidate()
	case ID_WORKERS_2:
		s.settings.CPUWorkers = 2
		s.uiStatus = "CPU workers: 2"
		s.invalidate()
	case ID_WORKERS_4:
		s.settings.CPUWorkers = 4
		s.uiStatus = "CPU workers: 4"
		s.invalidate()
	case ID_WORKERS_8:
		s.settings.CPUWorkers = 8
		s.uiStatus = "CPU workers: 8"
		s.invalidate()
	case ID_WORKERS_16:
		s.settings.CPUWorkers = 16
		s.uiStatus = "CPU workers: 16"
		s.invalidate()
	case ID_WORKERS_32:
		s.settings.CPUWorkers = 32
		s.uiStatus = "CPU workers: 32"
		s.invalidate()
	case ID_ADAPTIVE_SAMPLING:
		s.settings.AdaptiveSampling = !s.settings.AdaptiveSampling
		s.uiStatus = fmt.Sprintf("Adaptive sampling: %v", s.settings.AdaptiveSampling)
		s.invalidate()
	case ID_ADAPT_LOOSE:
		s.settings.AdaptiveThreshold = 0.002
		s.uiStatus = "Adaptive threshold: Loose"
		s.invalidate()
	case ID_ADAPT_MED:
		s.settings.AdaptiveThreshold = 0.0006
		s.uiStatus = "Adaptive threshold: Balanced"
		s.invalidate()
	case ID_ADAPT_TIGHT:
		s.settings.AdaptiveThreshold = 0.00018
		s.uiStatus = "Adaptive threshold: Tight"
		s.invalidate()
	case ID_BVH_MEDIAN:
		s.settings.BVHMode = "Median"
		s.uiStatus = "BVH: Median"
		s.invalidate()
	case ID_BVH_SAH8:
		s.settings.BVHMode, s.settings.BVHBins = "SAH", 8
		s.uiStatus = "BVH: SAH 8 bins"
		s.invalidate()
	case ID_BVH_SAH16:
		s.settings.BVHMode, s.settings.BVHBins = "SAH", 16
		s.uiStatus = "BVH: SAH 16 bins"
		s.invalidate()
	case ID_BVH_SAH32:
		s.settings.BVHMode, s.settings.BVHBins = "SAH", 32
		s.uiStatus = "BVH: SAH 32 bins"
		s.invalidate()
	case ID_LEAF_2:
		s.settings.BVHLeafSize = 2
		s.uiStatus = "BVH leaf size: 2"
		s.invalidate()
	case ID_LEAF_4:
		s.settings.BVHLeafSize = 4
		s.uiStatus = "BVH leaf size: 4"
		s.invalidate()
	case ID_LEAF_8:
		s.settings.BVHLeafSize = 8
		s.uiStatus = "BVH leaf size: 8"
		s.invalidate()
	case ID_TILE_SCANLINE:
		s.settings.TileOrder = "Scanline"
		s.uiStatus = "Tile order: Scanline"
		s.invalidate()
	case ID_TILE_CENTER:
		s.settings.TileOrder = "Center"
		s.uiStatus = "Tile order: Center-first"
		s.invalidate()
	case ID_PROGRESSIVE_TOGGLE:
		s.settings.ProgressiveUpdates = !s.settings.ProgressiveUpdates
		s.uiStatus = fmt.Sprintf("Progressive updates: %v", s.settings.ProgressiveUpdates)
		s.invalidate()
	case ID_PUBLISH_15:
		s.settings.PublishHz = 15
		s.uiStatus = "Progressive publish batches: 15"
		s.invalidate()
	case ID_PUBLISH_30:
		s.settings.PublishHz = 30
		s.uiStatus = "Progressive publish batches: 30"
		s.invalidate()
	case ID_PUBLISH_60:
		s.settings.PublishHz = 60
		s.uiStatus = "Progressive publish batches: 60"
		s.invalidate()
	case ID_HYPER_LATENCY:
		s.settings.TileSize = 8
		s.settings.TileOrder = "Center"
		s.settings.PublishHz = 60
		s.settings.ProgressiveUpdates = true
		s.settings.BVHMode, s.settings.BVHBins, s.settings.BVHLeafSize = "SAH", 8, 4
		s.settings.AdaptiveSampling = true
		s.settings.AdaptiveThreshold = 0.0015
		s.uiStatus = "Hyperopt preset: Latency"
		s.invalidate()
	case ID_HYPER_THROUGHPUT:
		s.settings.TileSize = 16
		s.settings.TileOrder = "Center"
		s.settings.PublishHz = 15
		s.settings.ProgressiveUpdates = false
		s.settings.BVHMode, s.settings.BVHBins, s.settings.BVHLeafSize = "SAH", 16, 8
		s.settings.AdaptiveSampling = true
		s.settings.AdaptiveThreshold = 0.0006
		s.uiStatus = "Hyperopt preset: Throughput"
		s.invalidate()
	case ID_HYPER_FINAL:
		s.settings.TileSize = 16
		s.settings.TileOrder = "Center"
		s.settings.PublishHz = 30
		s.settings.ProgressiveUpdates = true
		s.settings.BVHMode, s.settings.BVHBins, s.settings.BVHLeafSize = "SAH", 32, 2
		s.settings.AdaptiveSampling = true
		s.settings.AdaptiveThreshold = 0.00018
		s.uiStatus = "Hyperopt preset: Final efficiency"
		s.invalidate()
	case ID_HYPER_MULTICORE:
		s.settings.CPUWorkers = 0
		s.settings.TileSize = 8
		s.settings.TileOrder = "Center"
		s.settings.PublishHz = 15
		s.settings.ProgressiveUpdates = false
		s.settings.BVHMode, s.settings.BVHBins, s.settings.BVHLeafSize = "SAH", 16, 4
		s.settings.AdaptiveSampling = true
		s.uiStatus = "Hyperopt preset: Multicore saturation"
		s.invalidate()
	case ID_SAMPLER_RANDOM:
		s.settings.Sampler = SamplerRandom
		s.uiStatus = "Sampler: Random"
		s.invalidate()
	case ID_SAMPLER_HALTON:
		s.settings.Sampler = SamplerHalton
		s.uiStatus = "Sampler: Halton"
		s.invalidate()
	case ID_SAMPLER_R2:
		s.settings.Sampler = SamplerR2
		s.uiStatus = "Sampler: R2"
		s.invalidate()
	case ID_MIS_TOGGLE:
		s.settings.MIS = !s.settings.MIS
		s.uiStatus = fmt.Sprintf("MIS: %v", s.settings.MIS)
		s.invalidate()
	case ID_POWER_LIGHT_TOGGLE:
		s.settings.PowerLightSampling = !s.settings.PowerLightSampling
		s.uiStatus = fmt.Sprintf("Power-weighted lights: %v", s.settings.PowerLightSampling)
		s.invalidate()
	case ID_CLAMP_OFF:
		s.settings.FireflyClamp = 0
		s.uiStatus = "Firefly clamp: Off"
		s.invalidate()
	case ID_CLAMP_12:
		s.settings.FireflyClamp = 12
		s.uiStatus = "Firefly clamp: 12"
		s.invalidate()
	case ID_CLAMP_24:
		s.settings.FireflyClamp = 24
		s.uiStatus = "Firefly clamp: 24"
		s.invalidate()
	case ID_CLAMP_48:
		s.settings.FireflyClamp = 48
		s.uiStatus = "Firefly clamp: 48"
		s.invalidate()
	case ID_RR_2:
		s.settings.RRDepth = 2
		s.uiStatus = "Russian roulette: bounce 2"
		s.invalidate()
	case ID_RR_4:
		s.settings.RRDepth = 4
		s.uiStatus = "Russian roulette: bounce 4"
		s.invalidate()
	case ID_RR_6:
		s.settings.RRDepth = 6
		s.uiStatus = "Russian roulette: bounce 6"
		s.invalidate()
	case ID_FXAA_OFF:
		s.settings.FXAA = 0
		s.invalidate()
	case ID_FXAA_LOW:
		s.settings.FXAA = 1
		s.invalidate()
	case ID_FXAA_MED:
		s.settings.FXAA = 2
		s.invalidate()
	case ID_FXAA_HIGH:
		s.settings.FXAA = 3
		s.invalidate()
	case ID_TAA_OFF:
		s.settings.TAA = 0
		s.invalidate()
	case ID_TAA_LOW:
		s.settings.TAA = 1
		s.invalidate()
	case ID_TAA_MED:
		s.settings.TAA = 2
		s.invalidate()
	case ID_TAA_HIGH:
		s.settings.TAA = 3
		s.invalidate()
	case ID_TSAA_OFF:
		s.settings.TSAA = 0
		s.invalidate()
	case ID_TSAA_LOW:
		s.settings.TSAA = 1
		s.invalidate()
	case ID_TSAA_MED:
		s.settings.TSAA = 2
		s.invalidate()
	case ID_TSAA_HIGH:
		s.settings.TSAA = 3
		s.invalidate()
	case ID_TXAA_OFF:
		s.settings.TXAA = 0
		s.invalidate()
	case ID_TXAA_LOW:
		s.settings.TXAA = 1
		s.invalidate()
	case ID_TXAA_MED:
		s.settings.TXAA = 2
		s.invalidate()
	case ID_TXAA_HIGH:
		s.settings.TXAA = 3
		s.invalidate()
	case ID_MSAA_1:
		s.settings.MSAA = 1
		s.invalidate()
	case ID_MSAA_2:
		s.settings.MSAA = 2
		s.invalidate()
	case ID_MSAA_4:
		s.settings.MSAA = 4
		s.invalidate()
	case ID_MSAA_8:
		s.settings.MSAA = 8
		s.invalidate()
	case ID_HBAO_OFF:
		s.settings.HBAO = 0
		s.invalidate()
	case ID_HBAO_LOW:
		s.settings.HBAO = 1
		s.settings.HBAOPlus = 0
		s.invalidate()
	case ID_HBAO_MED:
		s.settings.HBAO = 2
		s.settings.HBAOPlus = 0
		s.invalidate()
	case ID_HBAO_HIGH:
		s.settings.HBAO = 3
		s.settings.HBAOPlus = 0
		s.invalidate()
	case ID_HBAOP_OFF:
		s.settings.HBAOPlus = 0
		s.invalidate()
	case ID_HBAOP_LOW:
		s.settings.HBAOPlus = 1
		s.settings.HBAO = 0
		s.invalidate()
	case ID_HBAOP_MED:
		s.settings.HBAOPlus = 2
		s.settings.HBAO = 0
		s.invalidate()
	case ID_HBAOP_HIGH:
		s.settings.HBAOPlus = 3
		s.settings.HBAO = 0
		s.invalidate()
	case ID_BLOOM_OFF:
		s.settings.Bloom = 0
		s.invalidate()
	case ID_BLOOM_LOW:
		s.settings.Bloom = 1
		s.invalidate()
	case ID_BLOOM_MED:
		s.settings.Bloom = 2
		s.invalidate()
	case ID_BLOOM_HIGH:
		s.settings.Bloom = 3
		s.invalidate()
	case ID_HDR_OFF:
		s.settings.HDR = 0
		s.invalidate()
	case ID_HDR_REINHARD:
		s.settings.HDR = 1
		s.invalidate()
	case ID_HDR_FILMIC:
		s.settings.HDR = 2
		s.invalidate()
	case ID_HDR_ACES:
		s.settings.HDR = 3
		s.invalidate()
	case ID_DEBUG_BEAUTY:
		s.settings.DebugView = DebugBeauty
		s.uiStatus = "Debug view: Beauty"
		s.invalidate()
	case ID_DEBUG_ALBEDO:
		s.settings.DebugView = DebugAlbedo
		s.uiStatus = "Debug view: Albedo"
		s.invalidate()
	case ID_DEBUG_NORMAL:
		s.settings.DebugView = DebugNormal
		s.uiStatus = "Debug view: Normals"
		s.invalidate()
	case ID_DEBUG_DEPTH:
		s.settings.DebugView = DebugDepth
		s.uiStatus = "Debug view: Depth"
		s.invalidate()
	case ID_DEBUG_AO:
		s.settings.DebugView = DebugAO
		s.uiStatus = "Debug view: AO"
		s.invalidate()
	case ID_DEBUG_HEAT:
		s.settings.DebugView = DebugHeat
		s.uiStatus = "Debug view: Heatmap"
		s.invalidate()
	case ID_OBJECT_EDIT:
		s.objectEdit = !s.objectEdit
		s.uiStatus = fmt.Sprintf("Object edit mode: %v", s.objectEdit)
		s.invalidate()
	case ID_OBJECT_DIFFUSE:
		s.addPresetObject("Diffuse Sphere")
	case ID_OBJECT_METAL:
		s.addPresetObject("Metal Sphere")
	case ID_OBJECT_GLASS:
		s.addPresetObject("Glass Sphere")
	case ID_OBJECT_LIGHT:
		s.addPresetObject("Light Sphere")
	case ID_OBJECT_CUBE:
		s.addPresetObject("Cube")
	case ID_CAMERA_RESET:
		s.camera = s.defaultCamera
		s.maybeAutoRender()
	case ID_CAMERA_LEFT:
		s.editCamera(func(c *Camera) { c.Orbit(-10, 0) })
	case ID_CAMERA_RIGHT:
		s.editCamera(func(c *Camera) { c.Orbit(10, 0) })
	case ID_CAMERA_UP:
		s.editCamera(func(c *Camera) { c.Orbit(0, 8) })
	case ID_CAMERA_DOWN:
		s.editCamera(func(c *Camera) { c.Orbit(0, -8) })
	case ID_CAMERA_NEAR:
		s.editCamera(func(c *Camera) { c.Dolly(-.8) })
	case ID_CAMERA_FAR:
		s.editCamera(func(c *Camera) { c.Dolly(.8) })
	case ID_SPEED_FINE:
		s.setCameraSpeed("Fine")
	case ID_SPEED_NORMAL:
		s.setCameraSpeed("Normal")
	case ID_SPEED_FAST:
		s.setCameraSpeed("Fast")
	case ID_VIEW_LEFT:
		s.rotate(-1)
	case ID_VIEW_RIGHT:
		s.rotate(1)
	case ID_VIEW_RESET:
		s.rotation = 0
		s.uiStatus = "Render rotation reset"
		s.invalidate()
	case ID_BENCH_QUICK:
		s.launchBenchmark("quick", "Quick CPU")
	case ID_BENCH_FULL:
		s.launchBenchmark("full", "FULL GIGA SUITE")
	case ID_BENCH_SCALING:
		s.launchBenchmark("scaling", "Multicore scaling")
	case ID_BENCH_POST:
		s.launchBenchmark("post", "Post-processing")
	case ID_BENCH_INTEGRATORS:
		s.launchBenchmark("integrators", "Integrator comparison")
	case ID_BENCH_GPU:
		s.launchBenchmark("gpu", "GPU / OpenCL probe")
	case ID_BENCH_SHOW_LAST:
		s.showLastBenchmark()
	case ID_HELP_ABOUT:
		message(s.hwnd, "Beamcast Studio 5.9 Ultra Realism", "Beamcast Studio 5.9 Ultra Realism\n\nNative Win32 desktop renderer.\n\nLeft-drag viewport: orbit camera\nRight-drag: pan target\nMouse wheel: zoom\nF5: render\nEsc: cancel\n[ / ]: rotate final render\nCtrl+O: load OBJ\nCtrl+S: save PPM\n\nBuilt as a self-contained Windows executable.", MB_OK|MB_ICONINFORMATION)
	}
}

func shortText(v string, n int) string {
	if n < 4 || len(v) <= n {
		return v
	}
	return v[:n-3] + "..."
}

func (s *Studio) click(x, y int) {
	px := s.panelX()
	if x < px {
		return
	}
	ids := []int{
		ID_RENDER_START, ID_RENDER_CANCEL, ID_PANEL_QUALITY,
		ID_TOGGLE_DENOISE, ID_TOGGLE_AUTORENDER, ID_LIVE_PREVIEW,
		ID_CAMERA_LEFT, ID_CAMERA_RIGHT, ID_CAMERA_UP, ID_CAMERA_DOWN,
		ID_CAMERA_NEAR, ID_CAMERA_FAR, ID_CAMERA_RESET,
		ID_OBJECT_EDIT, ID_VIEW_LEFT, ID_VIEW_RIGHT, ID_VIEW_RESET,
	}
	for _, id := range ids {
		if inside(s.panelRect(id), x, y) {
			s.command(id)
			return
		}
	}
}

func (s *Studio) paint() {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(s.hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer pEndPaint.Call(s.hwnd, uintptr(unsafe.Pointer(&ps)))
	memdc, _, _ := pCreateCompatibleDC.Call(hdc)
	if memdc == 0 {
		return
	}
	defer pDeleteDC.Call(memdc)
	bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(maxInt(s.clientW, 1)), uintptr(maxInt(s.clientH, 1)))
	if bmp == 0 {
		return
	}
	defer pDeleteObject.Call(bmp)
	oldBmp, _, _ := pSelectObject.Call(memdc, bmp)
	defer pSelectObject.Call(memdc, oldBmp)
	drawdc := memdc
	client := RECT{0, 0, int32(s.clientW), int32(s.clientH)}
	fillRect(drawdc, client, rgb(20, 22, 26))
	vp := s.viewportRect()
	fillRect(drawdc, vp, rgb(14, 16, 20))
	meta := s.render.Meta()
	if meta.FrameSerial != s.cachedSerial || s.cachedRotation != s.rotation {
		w, h, pix := s.render.PixelsCopy()
		if len(pix) > 0 && w > 0 && h > 0 {
			rotPix, rw, rh := RotatePixels(pix, w, h, s.rotation)
			s.cachedWinPix = make([]uint32, len(rotPix))
			for i, v := range rotPix {
				s.cachedWinPix[i] = ((v & 0xff) << 16) | (v & 0xff00) | ((v >> 16) & 0xff)
			}
			s.cachedImageW, s.cachedImageH = rw, rh
		} else {
			s.cachedWinPix = nil
			s.cachedImageW, s.cachedImageH = 0, 0
		}
		s.cachedSerial = meta.FrameSerial
		s.cachedRotation = s.rotation
	}
	if len(s.cachedWinPix) > 0 && s.cachedImageW > 0 && s.cachedImageH > 0 {
		rw, rh := s.cachedImageW, s.cachedImageH
		availW := int(vp.Right-vp.Left) - 28
		availH := int(vp.Bottom-vp.Top) - 28
		scale := math.Min(float64(availW)/float64(rw), float64(availH)/float64(rh))
		if scale <= 0 {
			scale = 1
		}
		dw := maxInt(1, int(float64(rw)*scale))
		dh := maxInt(1, int(float64(rh)*scale))
		dx := (int(vp.Right) - dw) / 2
		dy := (int(vp.Bottom) - dh) / 2
		bi := BITMAPINFOHEADER{BiSize: uint32(unsafe.Sizeof(BITMAPINFOHEADER{})), BiWidth: int32(rw), BiHeight: int32(-rh), BiPlanes: 1, BiBitCount: 32, BiCompression: 0}
		pSetStretchBltMode.Call(drawdc, HALFTONE)
		pStretchDIBits.Call(drawdc, uintptr(dx), uintptr(dy), uintptr(dw), uintptr(dh), 0, 0, uintptr(rw), uintptr(rh), uintptr(unsafe.Pointer(&s.cachedWinPix[0])), uintptr(unsafe.Pointer(&bi)), DIB_RGB_COLORS, SRCCOPY)
	} else {
		textOut(drawdc, 36, 42, "No render yet. Press F5 or click Render.", rgb(176, 181, 190))
		textOut(drawdc, 36, 70, "Left-drag = orbit   Right-drag = pan   Wheel = zoom", rgb(115, 122, 134))
	}
	px := s.panelX()
	panel := RECT{int32(px), 0, int32(s.clientW), int32(s.clientH)}
	fillRect(drawdc, panel, rgb(29, 32, 38))
	textOut(drawdc, px+22, 18, "BEAMCAST STUDIO 5.9 ULTRA REALISM", rgb(240, 242, 246))
	textOut(drawdc, px+22, 36, "High-fidelity path tracing / GPU research studio", rgb(132, 140, 152))
	drawButton(drawdc, s.panelRect(ID_RENDER_START), func() string {
		if meta.Rendering {
			return "Rendering..."
		}
		return "RENDER"
	}(), !meta.Rendering)
	drawButton(drawdc, s.panelRect(ID_RENDER_CANCEL), "Cancel", meta.Rendering)

	textOut(drawdc, px+22, 146, "RENDER CONFIGURATION", rgb(132, 177, 255))
	drawButton(drawdc, s.panelRect(ID_PANEL_QUALITY), "Quality: "+s.settings.Quality, true)
	drawButton(drawdc, s.panelRect(ID_TOGGLE_DENOISE), fmt.Sprintf("Denoise: %v", s.settings.Denoise), true)
	drawButton(drawdc, s.panelRect(ID_TOGGLE_AUTORENDER), fmt.Sprintf("Auto: %v", s.autoRender), true)
	drawButton(drawdc, s.panelRect(ID_LIVE_PREVIEW), fmt.Sprintf("Realtime movement preview: %v", s.livePreview), true)
	textOut(drawdc, px+22, 284, fmt.Sprintf("%s | base %d spp | %d bounces", s.resolutionLabel(), s.settings.SPP, s.settings.Bounces), rgb(188, 193, 202))
	textOut(drawdc, px+22, 302, shortText(fmt.Sprintf("%s | photon %s | debug %s", s.integratorLabel(), s.photonLabel(), s.debugLabel()), 60), rgb(170, 175, 184))
	textOut(drawdc, px+22, 320, shortText(fmt.Sprintf("Backend %s | DLSS %s", s.backendLabel(), s.upscaleLabel()), 60), rgb(170, 175, 184))
	textOut(drawdc, px+22, 338, shortText(s.aaSummary(), 60), rgb(170, 175, 184))
	textOut(drawdc, px+22, 356, shortText(s.postSummary()+" | "+s.toneLabel(), 60), rgb(170, 175, 184))
	textOut(drawdc, px+22, 374, shortText(s.smoothnessLabel()+" | "+s.perfLabel()+" | "+s.transportLabel(), 60), rgb(116, 124, 136))

	textOut(drawdc, px+22, 392, "CAMERA", rgb(132, 177, 255))
	drawButton(drawdc, s.panelRect(ID_CAMERA_LEFT), "Orbit left", true)
	drawButton(drawdc, s.panelRect(ID_CAMERA_RIGHT), "Orbit right", true)
	drawButton(drawdc, s.panelRect(ID_CAMERA_UP), "Orbit up", true)
	drawButton(drawdc, s.panelRect(ID_CAMERA_DOWN), "Orbit down", true)
	drawButton(drawdc, s.panelRect(ID_CAMERA_NEAR), "Closer", true)
	drawButton(drawdc, s.panelRect(ID_CAMERA_FAR), "Farther", true)
	drawButton(drawdc, s.panelRect(ID_CAMERA_RESET), "Reset camera", true)

	textOut(drawdc, px+22, 570, "OBJECT EDITOR", rgb(132, 177, 255))
	drawButton(drawdc, s.panelRect(ID_OBJECT_EDIT), fmt.Sprintf("Object drag mode: %v", s.objectEdit), true)
	textOut(drawdc, px+22, 632, shortText(s.selectedObjectLabel(), 60), rgb(224, 226, 230))
	textOut(drawdc, px+22, 650, "Objects menu adds presets; edit mode drags the selection", rgb(130, 137, 149))

	textOut(drawdc, px+22, 670, "FINAL RENDER VIEW", rgb(132, 177, 255))
	drawButton(drawdc, s.panelRect(ID_VIEW_LEFT), "Rotate left", true)
	drawButton(drawdc, s.panelRect(ID_VIEW_RIGHT), "Rotate right", true)
	drawButton(drawdc, s.panelRect(ID_VIEW_RESET), "Reset rotation", true)

	sceneName := "None"
	prim := 0
	nodes := 0
	objects := 0
	if s.scene != nil {
		sceneName = filepath.Base(s.scene.Name)
		prim = len(s.scene.Primitives)
		nodes = len(s.scene.Nodes)
		objects = len(s.scene.Objects)
	}
	textOut(drawdc, px+22, 778, "SCENE / TELEMETRY", rgb(132, 177, 255))
	textOut(drawdc, px+22, 798, shortText(fmt.Sprintf("%s | prim %d | BVH %d | editable %d", sceneName, prim, nodes, objects), 60), rgb(224, 226, 230))
	textOut(drawdc, px+22, 818, fmt.Sprintf("Progress %.1f%% | %.3f s | %d rays", meta.Progress*100, meta.Seconds, meta.Rays), rgb(224, 226, 230))
	rps := 0.0
	if meta.Seconds > 0 {
		rps = float64(meta.Rays) / meta.Seconds
	}
	backendLine := meta.BackendUsed
	if meta.DeviceName != "" {
		backendLine += " | " + meta.DeviceName
	}
	textOut(drawdc, px+22, 838, shortText(fmt.Sprintf("%.2f Mray/s | %s", rps/1e6, backendLine), 60), rgb(146, 184, 255))
	bar := RECT{int32(px + 22), 858, int32(s.clientW - 22), 870}
	fillRect(drawdc, bar, rgb(48, 52, 58))
	bw := int(float64(int(bar.Right-bar.Left)) * clamp(meta.Progress, 0, 1))
	if bw > 0 {
		fillRect(drawdc, RECT{bar.Left, bar.Top, bar.Left + int32(bw), bar.Bottom}, rgb(74, 134, 232))
	}
	status := s.uiStatus
	if status == "" {
		status = meta.Status
	}
	textOut(drawdc, px+22, 882, shortText(status, 60), rgb(195, 199, 207))
	footer := RECT{0, int32(s.clientH - 28), int32(s.clientW), int32(s.clientH)}
	fillRect(drawdc, footer, rgb(34, 37, 43))
	textOut(drawdc, 12, s.clientH-21, "F5 Render | Esc Cancel | Ctrl+O OBJ | Ctrl+S PPM | Render=integrator/AA/post/perf | Benchmark=GIGA suite | Debug=beauty/albedo/normal/depth/AO | LMB orbit or drag object", rgb(205, 208, 214))
	pBitBlt.Call(hdc, 0, 0, uintptr(maxInt(s.clientW, 1)), uintptr(maxInt(s.clientH, 1)), drawdc, 0, 0, SRCCOPY)
}

func wndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	if app == nil {
		return defWindow(hwnd, msg, wparam, lparam)
	}
	switch msg {
	case WM_CREATE:
		app.hwnd = hwnd
		pSetTimer.Call(hwnd, 1, uintptr(1000/maxInt(1, app.presentHz)), 0)
		return 0
	case WM_SIZE:
		app.clientW = loword(lparam)
		app.clientH = int((lparam >> 16) & 0xffff)
		app.invalidate()
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		app.paint()
		return 0
	case WM_TIMER:
		if app.render != nil {
			m := app.render.Meta()
			if m.FrameSerial != app.cachedSerial || m.Rendering != app.lastTimerRunning {
				app.invalidate()
			}
			app.lastTimerRunning = m.Rendering
		}
		return 0
	case WM_BENCH_DONE:
		app.uiStatus = "GIGA benchmark complete"
		app.invalidate()
		app.showLastBenchmark()
		return 0
	case WM_COMMAND:
		app.command(loword(wparam))
		return 0
	case WM_KEYDOWN:
		ctrlState, _, _ := pGetKeyState.Call(VK_CONTROL)
		ctrl := int16(ctrlState&0xffff) < 0
		if ctrl {
			switch wparam {
			case 'O':
				app.command(ID_FILE_OPEN)
				return 0
			case 'S':
				app.command(ID_FILE_SAVE_PPM)
				return 0
			case 'N':
				app.command(ID_FILE_NEW)
				return 0
			}
		}
		switch wparam {
		case VK_F5:
			app.command(ID_RENDER_START)
		case VK_ESCAPE:
			app.command(ID_RENDER_CANCEL)
		case VK_OEM_4:
			app.rotate(-1)
		case VK_OEM_6:
			app.rotate(1)
		}
		return 0
	case WM_LBUTTONDOWN, WM_RBUTTONDOWN:
		x := signedLo(lparam)
		y := signedHi(lparam)
		if x < app.panelX() {
			if msg == WM_LBUTTONDOWN && app.objectEdit && app.scene != nil {
				idx, _, ok := app.scene.PickObject(app.viewportRay(x, y))
				if ok {
					app.selectedObject = idx
					app.dragging = true
					app.dragButton = 3
					app.lastX = x
					app.lastY = y
					app.uiStatus = "Dragging object: " + app.scene.Objects[idx].Name
					pSetCapture.Call(hwnd)
				} else {
					app.selectedObject = -1
					app.uiStatus = "No editable preset object under cursor"
					app.invalidate()
				}
			} else {
				app.dragging = true
				app.dragButton = func() int {
					if msg == WM_LBUTTONDOWN {
						return 1
					}
					return 2
				}()
				app.lastX = x
				app.lastY = y
				pSetCapture.Call(hwnd)
			}
		} else {
			app.click(x, y)
		}
		return 0
	case WM_MOUSEMOVE:
		if app.dragging {
			x := signedLo(lparam)
			y := signedHi(lparam)
			dx := x - app.lastX
			dy := y - app.lastY
			app.lastX = x
			app.lastY = y
			if app.dragButton == 3 && app.scene != nil && app.selectedObject >= 0 {
				app.cancelAndWait()
				f := app.camera.Target.Sub(app.camera.Position).Unit()
				right := f.Cross(app.camera.Up).Unit()
				up := right.Cross(f).Unit()
				dist := app.camera.Position.Sub(app.camera.Target).Len()
				delta := right.Mul(float64(dx) * dist * app.panScale).Add(up.Mul(float64(-dy) * dist * app.panScale))
				app.scene.TranslateObject(app.selectedObject, delta)
				app.scene.Refit()
				app.uiStatus = "Moving " + app.scene.Objects[app.selectedObject].Name + " | BVH refit"
				app.startPreviewRender()
			} else if app.dragButton == 1 {
				app.camera.Orbit(float64(dx)*app.orbitScale, float64(-dy)*(app.orbitScale*0.8))
				app.uiStatus = "Camera orbit preview"
				app.startPreviewRender()
			} else {
				dist := app.camera.Position.Sub(app.camera.Target).Len()
				app.camera.Pan(float64(-dx)*dist*app.panScale, float64(dy)*dist*app.panScale)
				app.uiStatus = "Camera pan preview"
				app.startPreviewRender()
			}
			app.invalidate()
		}
		return 0
	case WM_LBUTTONUP, WM_RBUTTONUP:
		if app.dragging {
			wasObject := app.dragButton == 3
			app.dragging = false
			app.dragButton = 0
			pReleaseCapture.Call()
			if wasObject && app.scene != nil {
				app.cancelAndWait()
				app.configureSceneAcceleration()
				app.scene.Build()
			}
			app.maybeAutoRender()
		}
		return 0
	case WM_MOUSEWHEEL:
		delta := int16((wparam >> 16) & 0xffff)
		app.camera.Dolly(-float64(delta) / 120 * app.dollyScale)
		app.uiStatus = "Camera zoom"
		app.maybeAutoRender()
		return 0
	case WM_CLOSE:
		app.cancelAndWait()
		pDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		pKillTimer.Call(hwnd, 1)
		pPostQuitMessage.Call(0)
		return 0
	}
	return defWindow(hwnd, msg, wparam, lparam)
}
func defWindow(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

func main() {
	runtime.LockOSThread()
	app = NewStudio()
	hinst, _, _ := pGetModuleHandleW.Call(0)
	class := ptr("BeamcastStudio56MulticoreHyperScale")
	cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	icon, _, _ := pLoadIconW.Call(0, IDI_APPLICATION)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), Style: CS_HREDRAW | CS_VREDRAW, LpfnWndProc: syscall.NewCallback(wndProc), HInstance: hinst, HIcon: icon, HCursor: cursor, HbrBackground: 0, LpszClassName: class, HIconSm: icon}
	r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 {
		message(0, "Beamcast Studio", "Could not register Win32 window class: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	title := ptr("Beamcast Studio 5.9 Ultra Realism")
	hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), WS_OVERLAPPEDWINDOW, CW_USEDEFAULT, CW_USEDEFAULT, 1600, 980, 0, 0, hinst, 0)
	if hwnd == 0 {
		message(0, "Beamcast Studio", "Could not create window: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	app.hwnd = hwnd
	menu := buildMenu()
	pSetMenu.Call(hwnd, menu)
	pShowWindow.Call(hwnd, SW_SHOW)
	pUpdateWindow.Call(hwnd)
	app.startRender()
	var msg MSG
	for {
		res, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(res) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
