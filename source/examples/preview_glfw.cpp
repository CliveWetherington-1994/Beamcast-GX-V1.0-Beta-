#include <algorithm>
#include <iostream>
#include <memory>
#include <vector>
#include <GLFW/glfw3.h>
#include "beamcast/beamcast.hpp"

int main() {
    using namespace beamcast;
    World world;
    auto ground=std::make_shared<Lambertian>(Color{0.65f,0.65f,0.68f});
    auto glass=std::make_shared<Dielectric>(1.5f);
    auto metal=std::make_shared<Metal>(Color{0.8f,0.82f,0.9f},0.05f);
    world.add(std::make_shared<Sphere>(Point3{0,-100.5f,-1},100,ground));
    world.add(std::make_shared<Sphere>(Point3{-0.65f,0,-1},0.5f,glass));
    world.add(std::make_shared<Sphere>(Point3{0.65f,0,-1},0.5f,metal));
    world.build_acceleration(Acceleration::BVH_SAH);

    RenderSettings s; s.width=640; s.height=360; s.samples_per_pixel=16; s.max_bounces=10;
    Camera cam(Point3{3,1.5f,2.5f},Point3{0,0,-1},Vec3{0,1,0},40.0f,16.0f/9.0f,0.04f,4.2f);
    auto rgba=render(world,cam,s).rgba8();

    if (!glfwInit()) return 1;
    glfwWindowHint(GLFW_CONTEXT_VERSION_MAJOR,2); glfwWindowHint(GLFW_CONTEXT_VERSION_MINOR,1);
    GLFWwindow* win=glfwCreateWindow(s.width,s.height,"Beamcast OpenGL Preview",nullptr,nullptr);
    if (!win) { glfwTerminate(); return 1; }
    glfwMakeContextCurrent(win); glfwSwapInterval(1);
    while (!glfwWindowShouldClose(win)) {
        int w,h; glfwGetFramebufferSize(win,&w,&h); glViewport(0,0,w,h); glClear(GL_COLOR_BUFFER_BIT);
        glRasterPos2f(-1.0f,-1.0f); glPixelZoom(static_cast<float>(w)/s.width,static_cast<float>(h)/s.height);
        glDrawPixels(s.width,s.height,GL_RGBA,GL_UNSIGNED_BYTE,rgba.data());
        glfwSwapBuffers(win); glfwPollEvents();
        if (glfwGetKey(win,GLFW_KEY_ESCAPE)==GLFW_PRESS) glfwSetWindowShouldClose(win,1);
    }
    glfwDestroyWindow(win); glfwTerminate();
}
