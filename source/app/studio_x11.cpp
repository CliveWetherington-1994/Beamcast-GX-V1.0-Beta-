#include "studio_core.hpp"

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>

#include <algorithm>
#include <chrono>
#include <cmath>
#include <cstring>
#include <functional>
#include <iostream>
#include <string>
#include <thread>
#include <vector>

namespace {
using namespace beamcast;
using namespace beamcast::studio;

constexpr int kMenuH = 30;
constexpr int kPanelW = 310;
constexpr int kRowH = 31;

struct MenuItem { std::string text; int command; };
struct Menu { std::string text; int x; int w; std::vector<MenuItem> items; };

enum Command {
    CMD_NONE=0, CMD_NEW, CMD_OPEN_OBJ, CMD_SAVE, CMD_EXIT,
    CMD_SCENE_SHOWCASE, CMD_SCENE_CORNELL, CMD_SCENE_STRESS, CMD_SCENE_CLEAR,
    CMD_RENDER, CMD_CANCEL, CMD_QUALITY,
    CMD_CAMERA_RESET, CMD_CAMERA_LEFT, CMD_CAMERA_RIGHT, CMD_CAMERA_UP, CMD_CAMERA_DOWN,
    CMD_CAMERA_NEAR, CMD_CAMERA_FAR,
    CMD_VIEW_ROTATE_LEFT, CMD_VIEW_ROTATE_RIGHT, CMD_VIEW_ROTATE_RESET,
    CMD_ABOUT
};

struct CachedImage {
    XImage* image{nullptr};
    std::shared_ptr<const Framebuffer> source;
    int width{0};
    int height{0};
    int rotation_quarters{0};
    ~CachedImage() {
        if (image) {
            delete[] image->data;
            image->data = nullptr;
            XDestroyImage(image);
        }
    }
};

unsigned long scale_mask(unsigned char value, unsigned long mask) {
    if (!mask) return 0;
    unsigned long m = mask;
    int shift = 0;
    while ((m & 1UL) == 0) {
        m >>= 1;
        ++shift;
    }
    const unsigned long maxv = m;
    return ((static_cast<unsigned long>(value) * maxv + 127UL) / 255UL) << shift;
}

std::vector<Menu> make_menus() {
    return {
        {"File", 8, 52, {{"New showcase",CMD_NEW},{"Open OBJ...",CMD_OPEN_OBJ},{"Save render...",CMD_SAVE},{"Exit",CMD_EXIT}}},
        {"Scene", 62, 62, {{"Material showcase",CMD_SCENE_SHOWCASE},{"Cornell-like room",CMD_SCENE_CORNELL},{"Stress grid",CMD_SCENE_STRESS},{"Clear",CMD_SCENE_CLEAR}}},
        {"Render",126, 66, {{"Start render   F5",CMD_RENDER},{"Cycle quality",CMD_QUALITY},{"Cancel   Esc",CMD_CANCEL}}},
        {"Camera",194, 70, {{"Reset camera",CMD_CAMERA_RESET},{"Orbit left",CMD_CAMERA_LEFT},{"Orbit right",CMD_CAMERA_RIGHT},{"Orbit up",CMD_CAMERA_UP},{"Orbit down",CMD_CAMERA_DOWN},{"Move closer",CMD_CAMERA_NEAR},{"Move farther",CMD_CAMERA_FAR}}},
        {"View",266, 50, {{"Rotate render left   [",CMD_VIEW_ROTATE_LEFT},{"Rotate render right   ]",CMD_VIEW_ROTATE_RIGHT},{"Reset view rotation",CMD_VIEW_ROTATE_RESET}}},
        {"Help",318, 52, {{"About Beamcast Studio",CMD_ABOUT}}}
    };
}

void draw_text(Display* d, Drawable w, GC gc, int x, int y, const std::string& s) {
    XDrawString(d, w, gc, x, y, s.c_str(), static_cast<int>(s.size()));
}

void fill(Display* d, Drawable w, GC gc, unsigned long color, int x, int y, unsigned int width, unsigned int height) {
    XSetForeground(d, gc, color);
    XFillRectangle(d, w, gc, x, y, width, height);
}

void outline(Display* d, Drawable w, GC gc, unsigned long color, int x, int y, unsigned int width, unsigned int height) {
    XSetForeground(d, gc, color);
    XDrawRectangle(d, w, gc, x, y, width, height);
}

std::string prompt_path(Display* d, Window parent, const std::string& title, const std::string& initial) {
    const int screen = DefaultScreen(d);
    Window win = XCreateSimpleWindow(d, parent, 120, 120, 680, 120, 1, BlackPixel(d, screen), WhitePixel(d, screen));
    XStoreName(d, win, title.c_str());
    XSelectInput(d, win, ExposureMask | KeyPressMask | StructureNotifyMask);
    Atom wmDelete = XInternAtom(d, "WM_DELETE_WINDOW", False);
    XSetWMProtocols(d, win, &wmDelete, 1);
    GC gc = XCreateGC(d, win, 0, nullptr);
    XMapRaised(d, win);
    std::string value = initial;
    bool done = false;
    bool accepted = false;
    while (!done) {
        XEvent e;
        XNextEvent(d, &e);
        if (e.type == Expose) {
            XClearWindow(d, win);
            XSetForeground(d, gc, BlackPixel(d, screen));
            draw_text(d, win, gc, 14, 27, title);
            XDrawRectangle(d, win, gc, 12, 42, 650, 28);
            draw_text(d, win, gc, 18, 62, value + "_");
            draw_text(d, win, gc, 14, 96, "Enter = accept    Esc = cancel");
        } else if (e.type == KeyPress) {
            KeySym ks = 0;
            char buf[64];
            int n = XLookupString(&e.xkey, buf, sizeof(buf), &ks, nullptr);
            if (ks == XK_Return || ks == XK_KP_Enter) {
                accepted = true;
                done = true;
            } else if (ks == XK_Escape) {
                done = true;
            } else if (ks == XK_BackSpace) {
                if (!value.empty()) value.pop_back();
            } else if (n > 0) {
                for (int i = 0; i < n; ++i) {
                    if (static_cast<unsigned char>(buf[i]) >= 32) value.push_back(buf[i]);
                }
            }
            XClearArea(d, win, 0, 0, 680, 120, True);
        } else if (e.type == ClientMessage && static_cast<Atom>(e.xclient.data.l[0]) == wmDelete) {
            done = true;
        }
    }
    XFreeGC(d, gc);
    XDestroyWindow(d, win);
    XFlush(d);
    return accepted ? value : std::string{};
}

void message_box(Display* d, Window parent, const std::string& title, const std::string& message) {
    const int screen = DefaultScreen(d);
    Window win = XCreateSimpleWindow(d, parent, 160, 160, 700, 150, 1, BlackPixel(d, screen), WhitePixel(d, screen));
    XStoreName(d, win, title.c_str());
    XSelectInput(d, win, ExposureMask | KeyPressMask | ButtonPressMask);
    GC gc = XCreateGC(d, win, 0, nullptr);
    XMapRaised(d, win);
    bool done = false;
    while (!done) {
        XEvent e;
        XNextEvent(d, &e);
        if (e.type == Expose) {
            XSetForeground(d, gc, BlackPixel(d, screen));
            draw_text(d, win, gc, 16, 34, title);
            draw_text(d, win, gc, 16, 68, message);
            XDrawRectangle(d, win, gc, 300, 96, 100, 32);
            draw_text(d, win, gc, 335, 117, "OK");
        } else if (e.type == KeyPress || e.type == ButtonPress) {
            done = true;
        }
    }
    XFreeGC(d, gc);
    XDestroyWindow(d, win);
    XFlush(d);
}

class X11Studio {
public:
    int run(bool smoke_test = false) {
        d_ = XOpenDisplay(nullptr);
        if (!d_) {
            std::cerr << "Beamcast Studio: could not open X11 display.\n";
            return 2;
        }
        screen_ = DefaultScreen(d_);
        visual_ = DefaultVisual(d_, screen_);
        depth_ = DefaultDepth(d_, screen_);
        win_ = XCreateSimpleWindow(d_, RootWindow(d_, screen_), 50, 50, width_, height_, 1,
                                   BlackPixel(d_, screen_), WhitePixel(d_, screen_));
        XStoreName(d_, win_, "Beamcast Studio 5.3");
        XSelectInput(d_, win_, ExposureMask | ButtonPressMask | KeyPressMask | StructureNotifyMask);
        wmDelete_ = XInternAtom(d_, "WM_DELETE_WINDOW", False);
        XSetWMProtocols(d_, win_, &wmDelete_, 1);
        gc_ = XCreateGC(d_, win_, 0, nullptr);
        menus_ = make_menus();
        XMapWindow(d_, win_);

        bool quit = false;
        const auto smoke_start = std::chrono::steady_clock::now();
        auto last = std::chrono::steady_clock::now();
        while (!quit) {
            while (XPending(d_)) {
                XEvent e;
                XNextEvent(d_, &e);
                if (e.type == Expose) {
                    redraw();
                } else if (e.type == ConfigureNotify) {
                    width_ = std::max(640, e.xconfigure.width);
                    height_ = std::max(420, e.xconfigure.height);
                    cache_.source.reset();
                    redraw();
                } else if (e.type == ClientMessage && static_cast<Atom>(e.xclient.data.l[0]) == wmDelete_) {
                    quit = true;
                } else if (e.type == ButtonPress) {
                    if (handle_click(e.xbutton.x, e.xbutton.y)) quit = true;
                    redraw();
                } else if (e.type == KeyPress) {
                    if (handle_key(e.xkey)) quit = true;
                    redraw();
                }
            }
            const auto now = std::chrono::steady_clock::now();
            if (now - last > std::chrono::milliseconds(100)) {
                redraw();
                last = now;
            }
            if (smoke_test && now - smoke_start > std::chrono::milliseconds(350)) quit = true;
            std::this_thread::sleep_for(std::chrono::milliseconds(8));
        }
        core_.cancel_render();
        core_.wait_for_render();
        if (gc_) XFreeGC(d_, gc_);
        if (win_) XDestroyWindow(d_, win_);
        XCloseDisplay(d_);
        return 0;
    }

private:
    Display* d_{nullptr};
    int screen_{0};
    Visual* visual_{nullptr};
    int depth_{0};
    Window win_{0};
    GC gc_{0};
    Atom wmDelete_{};
    int width_{1280};
    int height_{820};
    int open_menu_{-1};
    int view_rotation_quarters_{0};
    StudioCore core_;
    std::vector<Menu> menus_;
    CachedImage cache_;
    std::string ui_status_;

    void set_status(std::string s) { ui_status_ = std::move(s); }
    void safe(const std::function<void()>& fn) {
        try {
            fn();
        } catch (const std::exception& e) {
            set_status(e.what());
            message_box(d_, win_, "Beamcast Studio", e.what());
        }
    }

    void rotate_view(int delta_quarters) {
        view_rotation_quarters_ = (view_rotation_quarters_ + delta_quarters) % 4;
        if (view_rotation_quarters_ < 0) view_rotation_quarters_ += 4;
        cache_.source.reset();
        set_status("View rotation: " + std::to_string(view_rotation_quarters_ * 90) + " deg");
    }

    bool handle_key(XKeyEvent& e) {
        KeySym ks = XLookupKeysym(&e, 0);
        const bool ctrl = (e.state & ControlMask) != 0;
        if (ks == XK_F5) {
            safe([&] { core_.start_render(); });
            return false;
        }
        if (ks == XK_Escape) {
            core_.cancel_render();
            open_menu_ = -1;
            return false;
        }
        if (ctrl && (ks == XK_o || ks == XK_O)) {
            do_open();
            return false;
        }
        if (ctrl && (ks == XK_s || ks == XK_S)) {
            do_save();
            return false;
        }
        if (ks == XK_bracketleft) {
            rotate_view(-1);
            return false;
        }
        if (ks == XK_bracketright) {
            rotate_view(+1);
            return false;
        }
        return false;
    }

    bool handle_click(int x, int y) {
        if (y < kMenuH) {
            for (std::size_t i = 0; i < menus_.size(); ++i) {
                if (x >= menus_[i].x && x < menus_[i].x + menus_[i].w) {
                    open_menu_ = open_menu_ == static_cast<int>(i) ? -1 : static_cast<int>(i);
                    return false;
                }
            }
            open_menu_ = -1;
            return false;
        }
        if (open_menu_ >= 0) {
            const auto& m = menus_[static_cast<std::size_t>(open_menu_)];
            const int top = kMenuH;
            const int mw = 240;
            if (x >= m.x && x < m.x + mw && y >= top && y < top + static_cast<int>(m.items.size()) * kRowH) {
                const int idx = (y - top) / kRowH;
                const int cmd = m.items[static_cast<std::size_t>(idx)].command;
                open_menu_ = -1;
                return execute(cmd);
            }
            open_menu_ = -1;
        }
        if (x >= width_ - kPanelW) return handle_panel(x, y);
        return false;
    }

    bool handle_panel(int x, int y) {
        const int left = width_ - kPanelW + 15;
        const int right = width_ - 15;
        if (x < left || x > right) return false;
        if (y >= 80 && y < 116) {
            execute(CMD_RENDER);
            return false;
        }
        if (y >= 122 && y < 154) {
            execute(CMD_CANCEL);
            return false;
        }
        if (y >= 190 && y < 222) {
            execute(CMD_QUALITY);
            return false;
        }
        if (y >= 228 && y < 260) {
            cycle_resolution();
            return false;
        }
        if (y >= 266 && y < 298) {
            auto s = core_.settings();
            s.denoise = !s.denoise;
            core_.set_settings(s);
            return false;
        }
        if (y >= 304 && y < 336) {
            auto s = core_.settings();
            s.variable_rate = !s.variable_rate;
            core_.set_settings(s);
            return false;
        }
        if (y >= 384 && y < 416) {
            if (x < (left + right) / 2) core_.orbit_camera(-10, 0); else core_.orbit_camera(10, 0);
            return false;
        }
        if (y >= 422 && y < 454) {
            if (x < (left + right) / 2) core_.orbit_camera(0, 8); else core_.orbit_camera(0, -8);
            return false;
        }
        if (y >= 460 && y < 492) {
            if (x < (left + right) / 2) core_.dolly_camera(-0.8f); else core_.dolly_camera(0.8f);
            return false;
        }
        if (y >= 498 && y < 530) {
            core_.reset_camera();
            return false;
        }
        if (y >= 566 && y < 598) {
            if (x < (left + right) / 2) rotate_view(-1); else rotate_view(+1);
            return false;
        }
        if (y >= 604 && y < 636) {
            view_rotation_quarters_ = 0;
            cache_.source.reset();
            set_status("View rotation reset");
            return false;
        }
        return false;
    }

    void cycle_resolution() {
        auto s = core_.settings();
        const int widths[] = {640, 960, 1280, 1920};
        const int heights[] = {360, 540, 720, 1080};
        int idx = 0;
        for (int i = 0; i < 4; ++i) {
            if (s.width == widths[i] && s.height == heights[i]) idx = i;
        }
        idx = (idx + 1) % 4;
        s.width = widths[idx];
        s.height = heights[idx];
        core_.set_settings(s);
    }

    void do_open() {
        std::string p = prompt_path(d_, win_, "Open OBJ path", "");
        if (!p.empty()) safe([&] { core_.import_obj(p); });
    }

    void do_save() {
        std::string p = prompt_path(d_, win_, "Save render as PPM", "beamcast_render.ppm");
        if (!p.empty()) {
            safe([&] {
                core_.save_image(p);
                set_status("Saved: " + p);
            });
        }
    }

    bool execute(int cmd) {
        switch (cmd) {
            case CMD_NEW:
            case CMD_SCENE_SHOWCASE:
                safe([&] { core_.load_preset(ScenePreset::Showcase); });
                break;
            case CMD_OPEN_OBJ:
                do_open();
                break;
            case CMD_SAVE:
                do_save();
                break;
            case CMD_EXIT:
                return true;
            case CMD_SCENE_CORNELL:
                safe([&] { core_.load_preset(ScenePreset::CornellLike); });
                break;
            case CMD_SCENE_STRESS:
                safe([&] { core_.load_preset(ScenePreset::StressGrid); });
                break;
            case CMD_SCENE_CLEAR:
                safe([&] { core_.clear_scene(); });
                break;
            case CMD_RENDER:
                safe([&] { core_.start_render(); });
                break;
            case CMD_CANCEL:
                core_.cancel_render();
                break;
            case CMD_QUALITY: {
                auto s = core_.settings();
                s.preset = next_preset(s.preset);
                core_.set_settings(s);
                break;
            }
            case CMD_CAMERA_RESET:
                core_.reset_camera();
                break;
            case CMD_CAMERA_LEFT:
                core_.orbit_camera(-10, 0);
                break;
            case CMD_CAMERA_RIGHT:
                core_.orbit_camera(10, 0);
                break;
            case CMD_CAMERA_UP:
                core_.orbit_camera(0, 8);
                break;
            case CMD_CAMERA_DOWN:
                core_.orbit_camera(0, -8);
                break;
            case CMD_CAMERA_NEAR:
                core_.dolly_camera(-0.8f);
                break;
            case CMD_CAMERA_FAR:
                core_.dolly_camera(0.8f);
                break;
            case CMD_VIEW_ROTATE_LEFT:
                rotate_view(-1);
                break;
            case CMD_VIEW_ROTATE_RIGHT:
                rotate_view(+1);
                break;
            case CMD_VIEW_ROTATE_RESET:
                view_rotation_quarters_ = 0;
                cache_.source.reset();
                set_status("View rotation reset");
                break;
            case CMD_ABOUT:
                message_box(d_, win_, "Beamcast Studio 5.3",
                            "Native desktop frontend for Beamcast GX.\nRotate the final render with [ and ].");
                break;
            default:
                break;
        }
        return false;
    }

    void rebuild_cache(const std::shared_ptr<const Framebuffer>& fb, int target_w, int target_h, int rotation_quarters) {
        if (!fb || target_w <= 0 || target_h <= 0) {
            cache_.source.reset();
            return;
        }
        if (cache_.source == fb && cache_.width == target_w && cache_.height == target_h && cache_.rotation_quarters == rotation_quarters) {
            return;
        }
        if (cache_.image) {
            delete[] cache_.image->data;
            cache_.image->data = nullptr;
            XDestroyImage(cache_.image);
            cache_.image = nullptr;
        }
        char* data = new char[static_cast<std::size_t>(target_w) * static_cast<std::size_t>(target_h) * 4]{};
        XImage* img = XCreateImage(d_, visual_, static_cast<unsigned int>(depth_), ZPixmap, 0, data,
                                   static_cast<unsigned int>(target_w), static_cast<unsigned int>(target_h), 32, 0);
        if (!img) {
            delete[] data;
            throw std::runtime_error("Could not create X11 preview image");
        }
        const auto rgba = fb->rgba8();
        const int sw = fb->width;
        const int sh = fb->height;
        const int dw = (rotation_quarters % 2 == 0) ? sw : sh;
        const int dh = (rotation_quarters % 2 == 0) ? sh : sw;
        for (int y = 0; y < target_h; ++y) {
            const int dy = std::clamp(y * dh / target_h, 0, dh - 1);
            for (int x = 0; x < target_w; ++x) {
                const int dx = std::clamp(x * dw / target_w, 0, dw - 1);
                int sx = dx;
                int sy = dy;
                switch (rotation_quarters & 3) {
                    case 0: sx = dx; sy = dy; break;
                    case 1: sx = dy; sy = sh - 1 - dx; break;
                    case 2: sx = sw - 1 - dx; sy = sh - 1 - dy; break;
                    case 3: sx = sw - 1 - dy; sy = dx; break;
                }
                sx = std::clamp(sx, 0, sw - 1);
                sy = std::clamp(sy, 0, sh - 1);
                const std::size_t i = 4 * (static_cast<std::size_t>(sy) * static_cast<std::size_t>(sw) + static_cast<std::size_t>(sx));
                const unsigned long px = scale_mask(rgba[i], visual_->red_mask)
                                       | scale_mask(rgba[i + 1], visual_->green_mask)
                                       | scale_mask(rgba[i + 2], visual_->blue_mask);
                XPutPixel(img, x, y, px);
            }
        }
        cache_.image = img;
        cache_.source = fb;
        cache_.width = target_w;
        cache_.height = target_h;
        cache_.rotation_quarters = rotation_quarters;
    }

    void draw_button(int x, int y, int w, int h, const std::string& text) {
        fill(d_, win_, gc_, 0xe8e8e8, x, y, w, h);
        outline(d_, win_, gc_, 0x707070, x, y, w, h);
        XSetForeground(d_, gc_, 0x202020);
        draw_text(d_, win_, gc_, x + 10, y + h / 2 + 5, text);
    }

    void redraw() {
        if (!d_ || !win_) return;
        fill(d_, win_, gc_, 0xf2f2f2, 0, 0, static_cast<unsigned int>(width_), static_cast<unsigned int>(height_));
        fill(d_, win_, gc_, 0x30343b, 0, 0, static_cast<unsigned int>(width_), kMenuH);
        XSetForeground(d_, gc_, 0xffffff);
        for (const auto& m : menus_) draw_text(d_, win_, gc_, m.x + 7, 20, m.text);

        const int viewport_w = std::max(1, width_ - kPanelW);
        const int viewport_h = std::max(1, height_ - kMenuH - 28);
        fill(d_, win_, gc_, 0x15171a, 0, kMenuH, viewport_w, viewport_h);
        auto fb = core_.image();
        if (fb && !fb->empty()) {
            const int rot = view_rotation_quarters_ & 3;
            const int image_w = (rot % 2 == 0) ? fb->width : fb->height;
            const int image_h = (rot % 2 == 0) ? fb->height : fb->width;
            float scale = std::min(static_cast<float>(viewport_w - 20) / image_w,
                                   static_cast<float>(viewport_h - 20) / image_h);
            scale = std::max(0.01f, scale);
            const int tw = std::max(1, static_cast<int>(image_w * scale));
            const int th = std::max(1, static_cast<int>(image_h * scale));
            try {
                rebuild_cache(fb, tw, th, rot);
                const int ox = (viewport_w - tw) / 2;
                const int oy = kMenuH + (viewport_h - th) / 2;
                XPutImage(d_, win_, gc_, cache_.image, 0, 0, ox, oy,
                          static_cast<unsigned int>(tw), static_cast<unsigned int>(th));
            } catch (const std::exception& e) {
                set_status(e.what());
            }
        } else {
            XSetForeground(d_, gc_, 0xa0a0a0);
            draw_text(d_, win_, gc_, 30, kMenuH + 42, "No render yet. Press F5 or click Render.");
        }

        const int px = width_ - kPanelW;
        fill(d_, win_, gc_, 0xf7f7f7, px, kMenuH, kPanelW, height_ - kMenuH);
        outline(d_, win_, gc_, 0xc0c0c0, px, kMenuH, kPanelW - 1, height_ - kMenuH - 1);
        const auto snap = core_.snapshot();
        const auto settings = core_.settings();
        const auto cam = core_.camera();
        XSetForeground(d_, gc_, 0x202020);
        draw_text(d_, win_, gc_, px + 15, 55, "BEAMCAST STUDIO 5.3");
        draw_button(px + 15, 80, kPanelW - 30, 36, snap.rendering ? "Rendering..." : "Render");
        draw_button(px + 15, 122, kPanelW - 30, 32, "Cancel");
        draw_text(d_, win_, gc_, px + 15, 177, "RENDER SETTINGS");
        draw_button(px + 15, 190, kPanelW - 30, 32, "Quality: " + preset_name(settings.preset));
        draw_button(px + 15, 228, kPanelW - 30, 32, "Resolution: " + std::to_string(settings.width) + " x " + std::to_string(settings.height));
        draw_button(px + 15, 266, kPanelW - 30, 32, std::string("Denoise: ") + (settings.denoise ? "On" : "Off"));
        draw_button(px + 15, 304, kPanelW - 30, 32, std::string("Variable rate: ") + (settings.variable_rate ? "On" : "Off"));

        draw_text(d_, win_, gc_, px + 15, 368, "CAMERA");
        draw_button(px + 15, 384, (kPanelW - 36) / 2, 32, "Orbit left");
        draw_button(px + 21 + (kPanelW - 36) / 2, 384, (kPanelW - 36) / 2, 32, "Orbit right");
        draw_button(px + 15, 422, (kPanelW - 36) / 2, 32, "Orbit up");
        draw_button(px + 21 + (kPanelW - 36) / 2, 422, (kPanelW - 36) / 2, 32, "Orbit down");
        draw_button(px + 15, 460, (kPanelW - 36) / 2, 32, "Closer");
        draw_button(px + 21 + (kPanelW - 36) / 2, 460, (kPanelW - 36) / 2, 32, "Farther");
        draw_button(px + 15, 498, kPanelW - 30, 32, "Reset camera");

        draw_text(d_, win_, gc_, px + 15, 550, "VIEW");
        draw_button(px + 15, 566, (kPanelW - 36) / 2, 32, "Rotate left");
        draw_button(px + 21 + (kPanelW - 36) / 2, 566, (kPanelW - 36) / 2, 32, "Rotate right");
        draw_button(px + 15, 604, kPanelW - 30, 32, "Reset view rotation");
        draw_text(d_, win_, gc_, px + 15, 656, "View angle: " + std::to_string((view_rotation_quarters_ & 3) * 90) + " deg");

        draw_text(d_, win_, gc_, px + 15, 686, "SCENE");
        draw_text(d_, win_, gc_, px + 15, 706, "Primitives: " + std::to_string(snap.primitive_count));
        std::string scene = snap.scene_name;
        if (scene.size() > 38) scene = "..." + scene.substr(scene.size() - 35);
        draw_text(d_, win_, gc_, px + 15, 726, scene);
        draw_text(d_, win_, gc_, px + 15, 746, "FOV: " + std::to_string(static_cast<int>(cam.vertical_fov_degrees)) + " deg");
        draw_text(d_, win_, gc_, px + 15, 774, "Render: " + std::to_string(snap.seconds).substr(0, 5) + " s");
        draw_text(d_, win_, gc_, px + 15, 794, "Rays/s: " + std::to_string(static_cast<long long>(snap.rays_per_second)));

        const int barx = px + 15;
        const int bary = height_ - 74;
        const int barw = kPanelW - 30;
        outline(d_, win_, gc_, 0x606060, barx, bary, barw, 16);
        fill(d_, win_, gc_, 0x5d9cec, barx + 1, bary + 1,
             static_cast<unsigned int>((barw - 1) * std::clamp(snap.progress, 0.0f, 1.0f)), 14);
        XSetForeground(d_, gc_, 0x202020);
        draw_text(d_, win_, gc_, px + 15, height_ - 34, ui_status_.empty() ? snap.status : ui_status_);

        fill(d_, win_, gc_, 0x25282e, 0, height_ - 28, width_, 28);
        XSetForeground(d_, gc_, 0xe0e0e0);
        draw_text(d_, win_, gc_, 10, height_ - 10,
                  "F5 Render   Esc Cancel   Ctrl+O Open OBJ   Ctrl+S Save PPM   [ / ] Rotate view");

        if (open_menu_ >= 0) {
            const auto& m = menus_[static_cast<std::size_t>(open_menu_)];
            const int mw = 240;
            const int mh = static_cast<int>(m.items.size()) * kRowH;
            fill(d_, win_, gc_, 0xf8f8f8, m.x, kMenuH, mw, mh);
            outline(d_, win_, gc_, 0x606060, m.x, kMenuH, mw, mh);
            XSetForeground(d_, gc_, 0x202020);
            for (std::size_t i = 0; i < m.items.size(); ++i) {
                draw_text(d_, win_, gc_, m.x + 10, kMenuH + 21 + static_cast<int>(i) * kRowH, m.items[i].text);
            }
        }
        XFlush(d_);
    }
};

} // namespace

int main(int argc, char** argv) {
    const bool smoke = argc > 1 && std::string(argv[1]) == "--smoke-test";
    try {
        return X11Studio{}.run(smoke);
    } catch (const std::exception& e) {
        std::cerr << "Beamcast Studio fatal error: " << e.what() << "\n";
        return 1;
    }
}
