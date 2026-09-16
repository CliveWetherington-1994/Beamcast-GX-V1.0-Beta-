#define NOMINMAX
#include "studio_core.hpp"

#include <windows.h>
#include <commdlg.h>

#include <algorithm>
#include <cstdint>
#include <functional>
#include <memory>
#include <string>
#include <vector>

namespace {
using namespace beamcast;
using namespace beamcast::studio;

constexpr int kPanelW = 330;

enum Id {
    ID_FILE_NEW = 100, ID_FILE_OPEN, ID_FILE_SAVE, ID_FILE_EXIT,
    ID_SCENE_SHOWCASE, ID_SCENE_CORNELL, ID_SCENE_STRESS, ID_SCENE_CLEAR,
    ID_RENDER_START, ID_RENDER_CANCEL, ID_RENDER_QUALITY,
    ID_CAMERA_RESET, ID_CAMERA_LEFT, ID_CAMERA_RIGHT, ID_CAMERA_UP, ID_CAMERA_DOWN, ID_CAMERA_NEAR, ID_CAMERA_FAR,
    ID_VIEW_ROTATE_LEFT, ID_VIEW_ROTATE_RIGHT, ID_VIEW_ROTATE_RESET,
    ID_HELP_ABOUT,
    ID_BTN_RENDER = 300, ID_BTN_CANCEL, ID_BTN_QUALITY, ID_BTN_RESOLUTION, ID_BTN_DENOISE, ID_BTN_VRS,
    ID_BTN_LEFT, ID_BTN_RIGHT, ID_BTN_UP, ID_BTN_DOWN, ID_BTN_NEAR, ID_BTN_FAR, ID_BTN_RESET,
    ID_BTN_VIEW_LEFT, ID_BTN_VIEW_RIGHT, ID_BTN_VIEW_RESET
};

std::string open_file_dialog(HWND owner) {
    char path[MAX_PATH]{};
    OPENFILENAMEA ofn{};
    ofn.lStructSize = sizeof(ofn);
    ofn.hwndOwner = owner;
    ofn.lpstrFile = path;
    ofn.nMaxFile = MAX_PATH;
    ofn.lpstrFilter = "Wavefront OBJ (*.obj)\0*.obj\0All files (*.*)\0*.*\0";
    ofn.nFilterIndex = 1;
    ofn.Flags = OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST | OFN_HIDEREADONLY;
    return GetOpenFileNameA(&ofn) ? std::string(path) : std::string{};
}

std::string save_file_dialog(HWND owner) {
    char path[MAX_PATH] = "beamcast_render.ppm";
    OPENFILENAMEA ofn{};
    ofn.lStructSize = sizeof(ofn);
    ofn.hwndOwner = owner;
    ofn.lpstrFile = path;
    ofn.nMaxFile = MAX_PATH;
    ofn.lpstrFilter = "PPM image (*.ppm)\0*.ppm\0All files (*.*)\0*.*\0";
    ofn.nFilterIndex = 1;
    ofn.lpstrDefExt = "ppm";
    ofn.Flags = OFN_OVERWRITEPROMPT | OFN_PATHMUSTEXIST;
    return GetSaveFileNameA(&ofn) ? std::string(path) : std::string{};
}

HMENU build_menu() {
    HMENU bar = CreateMenu();
    HMENU file = CreatePopupMenu();
    AppendMenuA(file, MF_STRING, ID_FILE_NEW, "&New showcase\tCtrl+N");
    AppendMenuA(file, MF_STRING, ID_FILE_OPEN, "&Open OBJ...\tCtrl+O");
    AppendMenuA(file, MF_STRING, ID_FILE_SAVE, "&Save render...\tCtrl+S");
    AppendMenuA(file, MF_SEPARATOR, 0, nullptr);
    AppendMenuA(file, MF_STRING, ID_FILE_EXIT, "E&xit");
    AppendMenuA(bar, MF_POPUP, reinterpret_cast<UINT_PTR>(file), "&File");

    HMENU scene = CreatePopupMenu();
    AppendMenuA(scene, MF_STRING, ID_SCENE_SHOWCASE, "Material showcase");
    AppendMenuA(scene, MF_STRING, ID_SCENE_CORNELL, "Cornell-like room");
    AppendMenuA(scene, MF_STRING, ID_SCENE_STRESS, "Stress grid");
    AppendMenuA(scene, MF_SEPARATOR, 0, nullptr);
    AppendMenuA(scene, MF_STRING, ID_SCENE_CLEAR, "Clear scene");
    AppendMenuA(bar, MF_POPUP, reinterpret_cast<UINT_PTR>(scene), "&Scene");

    HMENU render = CreatePopupMenu();
    AppendMenuA(render, MF_STRING, ID_RENDER_START, "&Start render\tF5");
    AppendMenuA(render, MF_STRING, ID_RENDER_QUALITY, "Cycle &quality");
    AppendMenuA(render, MF_STRING, ID_RENDER_CANCEL, "&Cancel\tEsc");
    AppendMenuA(bar, MF_POPUP, reinterpret_cast<UINT_PTR>(render), "&Render");

    HMENU camera = CreatePopupMenu();
    AppendMenuA(camera, MF_STRING, ID_CAMERA_RESET, "Reset camera");
    AppendMenuA(camera, MF_SEPARATOR, 0, nullptr);
    AppendMenuA(camera, MF_STRING, ID_CAMERA_LEFT, "Orbit left");
    AppendMenuA(camera, MF_STRING, ID_CAMERA_RIGHT, "Orbit right");
    AppendMenuA(camera, MF_STRING, ID_CAMERA_UP, "Orbit up");
    AppendMenuA(camera, MF_STRING, ID_CAMERA_DOWN, "Orbit down");
    AppendMenuA(camera, MF_STRING, ID_CAMERA_NEAR, "Move closer");
    AppendMenuA(camera, MF_STRING, ID_CAMERA_FAR, "Move farther");
    AppendMenuA(bar, MF_POPUP, reinterpret_cast<UINT_PTR>(camera), "&Camera");

    HMENU view = CreatePopupMenu();
    AppendMenuA(view, MF_STRING, ID_VIEW_ROTATE_LEFT, "Rotate render left");
    AppendMenuA(view, MF_STRING, ID_VIEW_ROTATE_RIGHT, "Rotate render right");
    AppendMenuA(view, MF_STRING, ID_VIEW_ROTATE_RESET, "Reset render rotation");
    AppendMenuA(bar, MF_POPUP, reinterpret_cast<UINT_PTR>(view), "&View");

    HMENU help = CreatePopupMenu();
    AppendMenuA(help, MF_STRING, ID_HELP_ABOUT, "&About Beamcast Studio");
    AppendMenuA(bar, MF_POPUP, reinterpret_cast<UINT_PTR>(help), "&Help");
    return bar;
}

class WinStudio {
public:
    explicit WinStudio(HINSTANCE instance) : instance_(instance) {}

    int run(int show) {
        WNDCLASSEXA wc{};
        wc.cbSize = sizeof(wc);
        wc.lpfnWndProc = &WinStudio::wnd_proc_static;
        wc.hInstance = instance_;
        wc.hCursor = LoadCursor(nullptr, IDC_ARROW);
        wc.hbrBackground = reinterpret_cast<HBRUSH>(COLOR_WINDOW + 1);
        wc.lpszClassName = "BeamcastStudio43";
        wc.hIcon = LoadIcon(nullptr, IDI_APPLICATION);
        wc.hIconSm = wc.hIcon;
        if (!RegisterClassExA(&wc)) return 1;

        hwnd_ = CreateWindowExA(0, wc.lpszClassName, "Beamcast Studio 5.3", WS_OVERLAPPEDWINDOW | WS_CLIPCHILDREN,
                                CW_USEDEFAULT, CW_USEDEFAULT, 1400, 900, nullptr, build_menu(), instance_, this);
        if (!hwnd_) return 2;
        ShowWindow(hwnd_, show);
        UpdateWindow(hwnd_);

        ACCEL keys[] = {
            {static_cast<BYTE>(FVIRTKEY | FCONTROL), 'N', ID_FILE_NEW},
            {static_cast<BYTE>(FVIRTKEY | FCONTROL), 'O', ID_FILE_OPEN},
            {static_cast<BYTE>(FVIRTKEY | FCONTROL), 'S', ID_FILE_SAVE},
            {static_cast<BYTE>(FVIRTKEY), VK_F5, ID_RENDER_START},
            {static_cast<BYTE>(FVIRTKEY), VK_ESCAPE, ID_RENDER_CANCEL},
            {static_cast<BYTE>(FVIRTKEY), VK_OEM_4, ID_VIEW_ROTATE_LEFT},
            {static_cast<BYTE>(FVIRTKEY), VK_OEM_6, ID_VIEW_ROTATE_RIGHT}
        };
        HACCEL accel = CreateAcceleratorTableA(keys, static_cast<int>(sizeof(keys) / sizeof(keys[0])));
        MSG msg{};
        while (GetMessageA(&msg, nullptr, 0, 0) > 0) {
            if (!TranslateAcceleratorA(hwnd_, accel, &msg)) {
                TranslateMessage(&msg);
                DispatchMessageA(&msg);
            }
        }
        if (accel) DestroyAcceleratorTable(accel);
        return static_cast<int>(msg.wParam);
    }

private:
    HINSTANCE instance_{};
    HWND hwnd_{};
    StudioCore core_;
    std::vector<HWND> controls_;
    HWND render_btn_{};
    HWND cancel_btn_{};
    HWND quality_btn_{};
    HWND resolution_btn_{};
    HWND denoise_btn_{};
    HWND vrs_btn_{};
    HWND scene_label_{};
    HWND telemetry_label_{};
    HWND status_label_{};
    HWND view_label_{};
    std::shared_ptr<const Framebuffer> cached_source_;
    std::vector<std::uint8_t> bgra_;
    int cached_view_width_{0};
    int cached_view_height_{0};
    int cached_rotation_quarters_{-1};
    int view_rotation_quarters_{0};

    static LRESULT CALLBACK wnd_proc_static(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
        WinStudio* self = reinterpret_cast<WinStudio*>(GetWindowLongPtrA(hwnd, GWLP_USERDATA));
        if (msg == WM_NCCREATE) {
            auto* cs = reinterpret_cast<CREATESTRUCTA*>(lp);
            self = static_cast<WinStudio*>(cs->lpCreateParams);
            SetWindowLongPtrA(hwnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(self));
            self->hwnd_ = hwnd;
        }
        return self ? self->wnd_proc(msg, wp, lp) : DefWindowProcA(hwnd, msg, wp, lp);
    }

    HWND button(int id, const char* text, int x, int y, int w, int h) {
        HWND c = CreateWindowExA(0, "BUTTON", text, WS_CHILD | WS_VISIBLE | BS_PUSHBUTTON,
                                 x, y, w, h, hwnd_, reinterpret_cast<HMENU>(static_cast<INT_PTR>(id)), instance_, nullptr);
        controls_.push_back(c);
        return c;
    }

    HWND label(const char* text, int x, int y, int w, int h) {
        HWND c = CreateWindowExA(0, "STATIC", text, WS_CHILD | WS_VISIBLE | SS_LEFT,
                                 x, y, w, h, hwnd_, nullptr, instance_, nullptr);
        controls_.push_back(c);
        return c;
    }

    void rotate_view(int delta_quarters) {
        view_rotation_quarters_ = (view_rotation_quarters_ + delta_quarters) % 4;
        if (view_rotation_quarters_ < 0) view_rotation_quarters_ += 4;
        cached_source_.reset();
        cached_rotation_quarters_ = -1;
        update_controls();
        InvalidateRect(hwnd_, nullptr, FALSE);
    }

    void create_controls() {
        render_btn_ = button(ID_BTN_RENDER, "Render", 10, 20, 290, 38);
        cancel_btn_ = button(ID_BTN_CANCEL, "Cancel", 10, 64, 290, 30);

        label("RENDER SETTINGS", 10, 112, 290, 20);
        quality_btn_ = button(ID_BTN_QUALITY, "Quality", 10, 136, 290, 32);
        resolution_btn_ = button(ID_BTN_RESOLUTION, "Resolution", 10, 174, 290, 32);
        denoise_btn_ = button(ID_BTN_DENOISE, "Denoise", 10, 212, 290, 32);
        vrs_btn_ = button(ID_BTN_VRS, "Variable rate", 10, 250, 290, 32);

        label("CAMERA", 10, 304, 290, 20);
        button(ID_BTN_LEFT, "Orbit left", 10, 328, 140, 30);
        button(ID_BTN_RIGHT, "Orbit right", 160, 328, 140, 30);
        button(ID_BTN_UP, "Orbit up", 10, 364, 140, 30);
        button(ID_BTN_DOWN, "Orbit down", 160, 364, 140, 30);
        button(ID_BTN_NEAR, "Closer", 10, 400, 140, 30);
        button(ID_BTN_FAR, "Farther", 160, 400, 140, 30);
        button(ID_BTN_RESET, "Reset camera", 10, 436, 290, 30);

        label("VIEW", 10, 486, 290, 20);
        button(ID_BTN_VIEW_LEFT, "Rotate left", 10, 510, 140, 30);
        button(ID_BTN_VIEW_RIGHT, "Rotate right", 160, 510, 140, 30);
        button(ID_BTN_VIEW_RESET, "Reset view rotation", 10, 546, 290, 30);
        view_label_ = label("View angle: 0 deg", 10, 582, 290, 20);

        label("SCENE", 10, 616, 290, 20);
        scene_label_ = label("", 10, 640, 290, 54);
        label("TELEMETRY", 10, 706, 290, 20);
        telemetry_label_ = label("", 10, 730, 290, 62);
        status_label_ = label("Ready", 10, 800, 290, 50);

        SetTimer(hwnd_, 1, 100, nullptr);
        layout_controls();
        update_controls();
    }

    void layout_controls() {
        RECT r{};
        GetClientRect(hwnd_, &r);
        const int x = std::max(0, (r.right - r.left) - kPanelW);
        for (HWND c : controls_) {
            RECT cr{};
            GetWindowRect(c, &cr);
            POINT p{cr.left, cr.top};
            ScreenToClient(hwnd_, &p);
            SetWindowPos(c, nullptr, x + (p.x < kPanelW ? p.x : 10), p.y, 0, 0, SWP_NOSIZE | SWP_NOZORDER);
        }
    }

    void safe(const std::function<void()>& fn) {
        try {
            fn();
        } catch (const std::exception& e) {
            MessageBoxA(hwnd_, e.what(), "Beamcast Studio", MB_OK | MB_ICONERROR);
        }
    }

    void cycle_resolution() {
        auto s = core_.settings();
        const int ws[] = {640, 960, 1280, 1920};
        const int hs[] = {360, 540, 720, 1080};
        int idx = 0;
        for (int i = 0; i < 4; ++i) {
            if (s.width == ws[i] && s.height == hs[i]) idx = i;
        }
        idx = (idx + 1) % 4;
        s.width = ws[idx];
        s.height = hs[idx];
        core_.set_settings(s);
    }

    void do_open() {
        auto p = open_file_dialog(hwnd_);
        if (!p.empty()) safe([&] { core_.import_obj(p); });
    }

    void do_save() {
        auto p = save_file_dialog(hwnd_);
        if (!p.empty()) safe([&] { core_.save_image(p); });
    }

    void command(int id) {
        switch (id) {
            case ID_FILE_NEW:
            case ID_SCENE_SHOWCASE:
                safe([&] { core_.load_preset(ScenePreset::Showcase); });
                break;
            case ID_FILE_OPEN:
                do_open();
                break;
            case ID_FILE_SAVE:
                do_save();
                break;
            case ID_FILE_EXIT:
                PostMessageA(hwnd_, WM_CLOSE, 0, 0);
                break;
            case ID_SCENE_CORNELL:
                safe([&] { core_.load_preset(ScenePreset::CornellLike); });
                break;
            case ID_SCENE_STRESS:
                safe([&] { core_.load_preset(ScenePreset::StressGrid); });
                break;
            case ID_SCENE_CLEAR:
                safe([&] { core_.clear_scene(); });
                break;
            case ID_RENDER_START:
            case ID_BTN_RENDER:
                safe([&] { core_.start_render(); });
                break;
            case ID_RENDER_CANCEL:
            case ID_BTN_CANCEL:
                core_.cancel_render();
                break;
            case ID_RENDER_QUALITY:
            case ID_BTN_QUALITY: {
                auto s = core_.settings();
                s.preset = next_preset(s.preset);
                core_.set_settings(s);
                break;
            }
            case ID_BTN_RESOLUTION:
                cycle_resolution();
                break;
            case ID_BTN_DENOISE: {
                auto s = core_.settings();
                s.denoise = !s.denoise;
                core_.set_settings(s);
                break;
            }
            case ID_BTN_VRS: {
                auto s = core_.settings();
                s.variable_rate = !s.variable_rate;
                core_.set_settings(s);
                break;
            }
            case ID_CAMERA_RESET:
            case ID_BTN_RESET:
                core_.reset_camera();
                break;
            case ID_CAMERA_LEFT:
            case ID_BTN_LEFT:
                core_.orbit_camera(-10, 0);
                break;
            case ID_CAMERA_RIGHT:
            case ID_BTN_RIGHT:
                core_.orbit_camera(10, 0);
                break;
            case ID_CAMERA_UP:
            case ID_BTN_UP:
                core_.orbit_camera(0, 8);
                break;
            case ID_CAMERA_DOWN:
            case ID_BTN_DOWN:
                core_.orbit_camera(0, -8);
                break;
            case ID_CAMERA_NEAR:
            case ID_BTN_NEAR:
                core_.dolly_camera(-0.8f);
                break;
            case ID_CAMERA_FAR:
            case ID_BTN_FAR:
                core_.dolly_camera(0.8f);
                break;
            case ID_VIEW_ROTATE_LEFT:
            case ID_BTN_VIEW_LEFT:
                rotate_view(-1);
                return;
            case ID_VIEW_ROTATE_RIGHT:
            case ID_BTN_VIEW_RIGHT:
                rotate_view(+1);
                return;
            case ID_VIEW_ROTATE_RESET:
            case ID_BTN_VIEW_RESET:
                view_rotation_quarters_ = 0;
                cached_source_.reset();
                cached_rotation_quarters_ = -1;
                break;
            case ID_HELP_ABOUT:
                MessageBoxA(hwnd_,
                            "Beamcast Studio 5.3\nNative desktop frontend for Beamcast GX.\n\nF5: render\nEsc: cancel\nCtrl+O: open OBJ\nCtrl+S: save PPM\n[ and ]: rotate the final render",
                            "About Beamcast Studio",
                            MB_OK | MB_ICONINFORMATION);
                break;
        }
        update_controls();
        InvalidateRect(hwnd_, nullptr, FALSE);
    }

    void update_controls() {
        if (!render_btn_) return;
        const auto snap = core_.snapshot();
        const auto s = core_.settings();
        SetWindowTextA(render_btn_, snap.rendering ? "Rendering..." : "Render");
        SetWindowTextA(quality_btn_, ("Quality: " + preset_name(s.preset)).c_str());
        SetWindowTextA(resolution_btn_, ("Resolution: " + std::to_string(s.width) + " x " + std::to_string(s.height)).c_str());
        SetWindowTextA(denoise_btn_, s.denoise ? "Denoise: On" : "Denoise: Off");
        SetWindowTextA(vrs_btn_, s.variable_rate ? "Variable rate: On" : "Variable rate: Off");
        SetWindowTextA(view_label_, ("View angle: " + std::to_string((view_rotation_quarters_ & 3) * 90) + " deg").c_str());

        std::string scene = "Primitives: " + std::to_string(snap.primitive_count) + "\r\n" + snap.scene_name;
        SetWindowTextA(scene_label_, scene.c_str());
        std::string tele = "Render: " + std::to_string(snap.seconds) + " s\r\nRays/s: "
                         + std::to_string(static_cast<long long>(snap.rays_per_second))
                         + "\r\nProgress: " + std::to_string(static_cast<int>(snap.progress * 100)) + "%";
        SetWindowTextA(telemetry_label_, tele.c_str());
        SetWindowTextA(status_label_, snap.status.c_str());
        EnableWindow(render_btn_, !snap.rendering);
        EnableWindow(cancel_btn_, snap.rendering);
    }

    void update_image_cache() {
        auto fb = core_.image();
        const int rot = view_rotation_quarters_ & 3;
        if (fb == cached_source_ && rot == cached_rotation_quarters_) return;
        cached_source_ = fb;
        cached_rotation_quarters_ = rot;
        cached_view_width_ = 0;
        cached_view_height_ = 0;
        bgra_.clear();
        if (!fb || fb->empty()) return;

        const auto rgba = fb->rgba8();
        const int sw = fb->width;
        const int sh = fb->height;
        cached_view_width_ = (rot % 2 == 0) ? sw : sh;
        cached_view_height_ = (rot % 2 == 0) ? sh : sw;
        bgra_.resize(static_cast<std::size_t>(cached_view_width_) * static_cast<std::size_t>(cached_view_height_) * 4);

        for (int y = 0; y < cached_view_height_; ++y) {
            for (int x = 0; x < cached_view_width_; ++x) {
                int sx = x;
                int sy = y;
                switch (rot) {
                    case 0: sx = x; sy = y; break;
                    case 1: sx = y; sy = sh - 1 - x; break;
                    case 2: sx = sw - 1 - x; sy = sh - 1 - y; break;
                    case 3: sx = sw - 1 - y; sy = x; break;
                }
                sx = std::clamp(sx, 0, sw - 1);
                sy = std::clamp(sy, 0, sh - 1);
                const std::size_t si = 4 * (static_cast<std::size_t>(sy) * static_cast<std::size_t>(sw) + static_cast<std::size_t>(sx));
                const std::size_t di = 4 * (static_cast<std::size_t>(y) * static_cast<std::size_t>(cached_view_width_) + static_cast<std::size_t>(x));
                bgra_[di] = rgba[si + 2];
                bgra_[di + 1] = rgba[si + 1];
                bgra_[di + 2] = rgba[si];
                bgra_[di + 3] = 255;
            }
        }
    }

    void paint() {
        PAINTSTRUCT ps{};
        HDC dc = BeginPaint(hwnd_, &ps);
        RECT r{};
        GetClientRect(hwnd_, &r);
        const int vw = std::max(1, r.right - kPanelW);
        RECT view{0, 0, vw, r.bottom};
        HBRUSH dark = CreateSolidBrush(RGB(22, 24, 28));
        FillRect(dc, &view, dark);
        DeleteObject(dark);
        update_image_cache();

        if (cached_source_ && !bgra_.empty()) {
            const int sw = cached_view_width_;
            const int sh = cached_view_height_;
            double scale = std::min((vw - 24.0) / sw, (r.bottom - 24.0) / sh);
            scale = std::max(0.01, scale);
            const int dw = std::max(1, static_cast<int>(sw * scale));
            const int dh = std::max(1, static_cast<int>(sh * scale));
            const int dx = (vw - dw) / 2;
            const int dy = (r.bottom - dh) / 2;
            BITMAPINFO bmi{};
            bmi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
            bmi.bmiHeader.biWidth = sw;
            bmi.bmiHeader.biHeight = -sh;
            bmi.bmiHeader.biPlanes = 1;
            bmi.bmiHeader.biBitCount = 32;
            bmi.bmiHeader.biCompression = BI_RGB;
            SetStretchBltMode(dc, HALFTONE);
            StretchDIBits(dc, dx, dy, dw, dh, 0, 0, sw, sh, bgra_.data(), &bmi, DIB_RGB_COLORS, SRCCOPY);
        } else {
            SetTextColor(dc, RGB(185, 185, 185));
            SetBkMode(dc, TRANSPARENT);
            TextOutA(dc, 28, 36, "No render yet. Press F5 or click Render.", 39);
        }

        const auto snap = core_.snapshot();
        const int barX = vw + 10;
        const int barY = 850;
        const int barW = 300;
        const int barH = 16;
        Rectangle(dc, barX, barY, barX + barW, barY + barH);
        HBRUSH b = CreateSolidBrush(RGB(71, 139, 229));
        RECT pr{barX + 1, barY + 1, barX + 1 + static_cast<int>((barW - 2) * std::clamp(snap.progress, 0.0f, 1.0f)), barY + barH - 1};
        FillRect(dc, &pr, b);
        DeleteObject(b);
        EndPaint(hwnd_, &ps);
    }

    LRESULT wnd_proc(UINT msg, WPARAM wp, LPARAM lp) {
        switch (msg) {
            case WM_CREATE:
                create_controls();
                return 0;
            case WM_SIZE:
                layout_controls();
                InvalidateRect(hwnd_, nullptr, TRUE);
                return 0;
            case WM_TIMER:
                update_controls();
                InvalidateRect(hwnd_, nullptr, FALSE);
                return 0;
            case WM_COMMAND:
                command(LOWORD(wp));
                return 0;
            case WM_KEYDOWN:
                if (wp == VK_F5) {
                    command(ID_RENDER_START);
                    return 0;
                }
                if (wp == VK_ESCAPE) {
                    command(ID_RENDER_CANCEL);
                    return 0;
                }
                if (wp == VK_OEM_4) {
                    command(ID_VIEW_ROTATE_LEFT);
                    return 0;
                }
                if (wp == VK_OEM_6) {
                    command(ID_VIEW_ROTATE_RIGHT);
                    return 0;
                }
                if ((GetKeyState(VK_CONTROL) & 0x8000) != 0) {
                    if (wp == 'O') {
                        command(ID_FILE_OPEN);
                        return 0;
                    }
                    if (wp == 'S') {
                        command(ID_FILE_SAVE);
                        return 0;
                    }
                    if (wp == 'N') {
                        command(ID_FILE_NEW);
                        return 0;
                    }
                }
                break;
            case WM_GETMINMAXINFO: {
                auto* m = reinterpret_cast<MINMAXINFO*>(lp);
                m->ptMinTrackSize.x = 1000;
                m->ptMinTrackSize.y = 900;
                return 0;
            }
            case WM_PAINT:
                paint();
                return 0;
            case WM_CLOSE:
                core_.cancel_render();
                core_.wait_for_render();
                DestroyWindow(hwnd_);
                return 0;
            case WM_DESTROY:
                KillTimer(hwnd_, 1);
                PostQuitMessage(0);
                return 0;
        }
        return DefWindowProcA(hwnd_, msg, wp, lp);
    }
};

} // namespace

int WINAPI WinMain(HINSTANCE instance, HINSTANCE, LPSTR, int show) {
    try {
        return WinStudio(instance).run(show);
    } catch (const std::exception& e) {
        MessageBoxA(nullptr, e.what(), "Beamcast Studio fatal error", MB_OK | MB_ICONERROR);
        return 1;
    }
}
