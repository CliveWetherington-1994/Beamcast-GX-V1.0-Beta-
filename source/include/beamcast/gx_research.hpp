#pragma once

#include <array>
#include <atomic>
#include <condition_variable>
#include <cstddef>
#include <cstdint>
#include <deque>
#include <functional>
#include <memory>
#include <mutex>
#include <optional>
#include <string>
#include <thread>
#include <unordered_map>
#include <vector>

#include "gx.hpp"
#include "gx_stream.hpp"

namespace beamcast::gx {

// ---------------- Native backend discovery / common runtime contract ----------------

enum class NativeBackend : std::uint8_t { CPU, OpenCL, Vulkan, CUDA, HIP };

struct BackendInfo {
    NativeBackend backend{NativeBackend::CPU};
    std::string name;
    bool runtime_library_present{false};
    bool compiled_support{false};
    bool kernel_source_present{false};
    std::uint32_t device_count{0};
    std::uint32_t compute_queue_count{0};
    std::string detail;
};

std::vector<BackendInfo> discover_native_backends();
const char* backend_name(NativeBackend backend);

struct DeviceQueueCompactionResult {
    std::vector<std::uint32_t> prefix;
    std::vector<std::uint32_t> live_indices;
};

// CPU reference for the exact prefix-sum/scatter contract used by the GPU kernels.
DeviceQueueCompactionResult compact_alive_reference(const std::vector<std::uint32_t>& flags);

std::vector<std::uint32_t> radix_sort_indices_by_key(const std::vector<std::uint32_t>& keys);
Scalar queue_coherence_score(const std::vector<std::uint32_t>& keys,const std::vector<std::uint32_t>& order);
struct QueueOrderingDecision {
    bool sort{false};
    Scalar before_coherence{0};
    Scalar after_coherence{0};
    double measured_sort_ns{0};
    double estimated_saved_ns{0};
};
QueueOrderingDecision evaluate_queue_ordering(const std::vector<std::uint32_t>& keys,
                                              double divergence_cost_ns_per_ray);

// ---------------- Wide compressed BVH8 ----------------

struct alignas(16) WideBvh8Node {
    // SoA quantized child bounds: 96 bytes for eight child AABBs.
    std::uint16_t bmin[3][8]{};
    std::uint16_t bmax[3][8]{};
    std::uint32_t index[8]{};      // child node index for internal, ref start for leaf
    std::uint16_t count[8]{};      // primitive count for leaf
    std::uint8_t kind[8]{};        // 0 unused, 1 internal, 2 leaf
    std::uint8_t child_count{0};
    std::uint8_t pad[7]{};
};

static_assert(sizeof(WideBvh8Node)==160,"WideBvh8Node ABI must remain 160 bytes");

class WideBvh8 {
    Point3 world_min_{};
    Vec3 world_extent_{1,1,1};
    std::vector<WideBvh8Node> nodes_;
    std::vector<hx::GpuPrimitiveRef> refs_;
    std::vector<hx::PackedSphere> spheres_;
    std::vector<hx::PackedTriangle> triangles_;

public:
    void build(const hx::PackedScene& scene);
    bool hit(const Ray& ray, Scalar t_min, Scalar t_max, hx::PackedHit& hit,
             hx::TraceStats* stats=nullptr) const;
    const std::vector<WideBvh8Node>& nodes() const { return nodes_; }
    std::size_t byte_size() const;
    Scalar average_child_occupancy() const;
    Point3 world_min() const { return world_min_; }
    Vec3 world_extent() const { return world_extent_; }
};

// ---------------- Full ReSTIR DI reference implementation ----------------

struct SurfacePoint {
    Point3 position{};
    Vec3 normal{};
    Color albedo{};
    Scalar depth{0};
    bool valid{false};
};

struct MotionVector {
    // Current-pixel -> previous-frame displacement in pixel units.
    Scalar x{0};
    Scalar y{0};
};

struct RestirLightSample {
    Point3 position{};
    Vec3 normal{};
    Color emission{};
    std::uint32_t primitive_type{0};
    std::uint32_t primitive_index{0};
    Scalar area{0};
    Scalar selection_pdf{0};
};

struct RestirReservoir {
    RestirLightSample sample{};
    Scalar weight_sum{0};
    Scalar selected_target{0};
    std::uint32_t M{0};
    bool valid{false};
};

struct RestirDIConfig {
    int initial_candidates{8};
    int spatial_neighbors{5};
    int spatial_radius{16};
    bool temporal_reuse{true};
    bool spatial_reuse{true};
    bool visibility_validation{true};
    Scalar normal_threshold{0.9f};
    Scalar relative_depth_threshold{0.05f};
    std::uint64_t seed{0x5245535449524449ULL};
};

struct RestirDIHistory {
    int width{0};
    int height{0};
    std::vector<RestirReservoir> reservoirs;
    std::vector<SurfacePoint> surfaces;
    void reset();
};

struct RestirDIResult {
    Framebuffer direct;
    std::vector<RestirReservoir> reservoirs;
    std::uint64_t fresh_candidates{0};
    std::uint64_t temporal_reuses{0};
    std::uint64_t spatial_reuses{0};
    std::uint64_t visibility_rays{0};
};

std::vector<SurfacePoint> surfaces_from_guides(const Guides& guides);
std::vector<MotionVector> motion_vectors_from_positions(const std::vector<Point3>& positions,
                                                        int width,
                                                        int height,
                                                        const Camera& current_camera,
                                                        const Camera& previous_camera);
RestirDIResult restir_di(const hx::PackedScene& scene,
                         int width,
                         int height,
                         const std::vector<SurfacePoint>& surfaces,
                         const std::vector<MotionVector>& current_to_previous,
                         const RestirDIHistory* previous,
                         const RestirDIConfig& config);
void update_restir_history(RestirDIHistory& history,
                           int width,
                           int height,
                           const std::vector<SurfacePoint>& surfaces,
                           const std::vector<RestirReservoir>& reservoirs);

// ---------------- Indirect reuse: radiance cache + directional path guiding ----------------

class RadianceCache {
    struct Cell {
        Color sum{};
        std::uint32_t count{0};
        std::uint32_t last_frame{0};
    };
    Scalar cell_size_{1};
    std::uint32_t frame_{0};
    std::unordered_map<std::uint64_t, Cell> cells_;
    std::uint64_t key(Point3 p) const;
public:
    explicit RadianceCache(Scalar cell_size=1.0f) : cell_size_(cell_size > 1e-6f ? cell_size : 1.0f) {}
    void begin_frame(std::uint32_t frame);
    void insert(Point3 p, Color radiance);
    std::optional<Color> lookup(Point3 p, std::uint32_t max_age=8) const;
    void prune(std::uint32_t max_age=32);
    std::size_t cell_count() const { return cells_.size(); }
};

class PathGuideGrid {
    struct Cell {
        std::array<Scalar,8> weight{{1,1,1,1,1,1,1,1}};
    };
    Scalar cell_size_{1};
    std::unordered_map<std::uint64_t, Cell> cells_;
    std::uint64_t key(Point3 p) const;
public:
    explicit PathGuideGrid(Scalar cell_size=1.0f) : cell_size_(cell_size > 1e-6f ? cell_size : 1.0f) {}
    void update(Point3 p, Vec3 direction, Scalar contribution);
    Vec3 sample(Point3 p, std::uint64_t& rng_state) const;
    Scalar pdf(Point3 p, Vec3 direction) const;
    std::size_t cell_count() const { return cells_.size(); }
};

// ---------------- Virtualized geometry ----------------

struct ClusterPage {
    AABB bounds{};
    std::size_t page{0};
    std::uint32_t triangle_count{0};
};

class ClusterBvh {
    struct Node {
        AABB bounds{};
        std::uint32_t left{0};
        std::uint32_t right{0};
        std::uint32_t first{0};
        std::uint32_t count{0};
        bool leaf{false};
    };
    std::vector<ClusterPage> pages_;
    std::vector<std::uint32_t> order_;
    std::vector<Node> nodes_;
    std::uint32_t build_node(std::uint32_t begin, std::uint32_t end);
public:
    void build(const PagedTriangleStore& store);
    std::vector<std::size_t> query(const Ray& ray, Scalar t_min, Scalar t_max) const;
    const std::vector<ClusterPage>& pages() const { return pages_; }
    std::size_t node_count() const { return nodes_.size(); }
};

class PageResidencyManager {
public:
    using UploadCallback = std::function<std::size_t(std::size_t,const std::vector<hx::PackedTriangle>&)>;
    using EvictCallback = std::function<void(std::size_t)>;

private:
    struct Entry {
        std::vector<hx::PackedTriangle> triangles;
        std::size_t host_bytes{0};
        std::size_t device_bytes{0};
        std::uint64_t stamp{0};
        bool loading{false};
        bool ready{false};
    };
    const PagedTriangleStore* store_{nullptr};
    std::size_t host_budget_{0};
    std::size_t device_budget_{0};
    UploadCallback upload_;
    EvictCallback evict_;
    mutable std::mutex mutex_;
    std::condition_variable cv_;
    std::condition_variable work_cv_;
    std::unordered_map<std::size_t,Entry> entries_;
    std::deque<std::size_t> pending_;
    std::thread worker_;
    bool stop_{false};
    std::uint64_t stamp_{0};
    std::size_t host_bytes_{0};
    std::size_t device_bytes_{0};
    void worker_loop();
    std::vector<std::size_t> evict_to_budget_locked(std::optional<std::size_t> protect={});

public:
    PageResidencyManager(const PagedTriangleStore& store,
                         std::size_t host_budget_bytes,
                         std::size_t device_budget_bytes=0,
                         UploadCallback upload={},
                         EvictCallback evict={});
    ~PageResidencyManager();
    PageResidencyManager(const PageResidencyManager&)=delete;
    PageResidencyManager& operator=(const PageResidencyManager&)=delete;
    void request(std::size_t page);
    bool wait(std::size_t page);
    std::optional<std::vector<hx::PackedTriangle>> get(std::size_t page);
    bool resident(std::size_t page) const;
    std::size_t host_resident_bytes() const;
    std::size_t device_resident_bytes() const;
    std::size_t resident_pages() const;
};

// ---------------- Reconstruction: TAA + optional tiny neural inference ----------------

struct ReconstructionGuides {
    int width{0};
    int height{0};
    std::vector<Vec3> normal;
    std::vector<Scalar> depth;
    std::vector<MotionVector> motion;
};

class TemporalAA {
    Framebuffer history_;
    ReconstructionGuides guides_;
    std::vector<std::uint16_t> history_length_;
public:
    void reset();
    Framebuffer resolve(const Framebuffer& current,
                        const ReconstructionGuides& guides,
                        int max_history=24,
                        Scalar normal_threshold=0.9f,
                        Scalar relative_depth_threshold=0.05f,
                        Scalar clamp_strength=1.25f);
};

struct TinyNeuralModel {
    static constexpr int kInputs=12;
    static constexpr int kHidden=16;
    std::array<Scalar,kHidden*kInputs> w1{};
    std::array<Scalar,kHidden> b1{};
    std::array<Scalar,3*kHidden> w2{};
    std::array<Scalar,3> b2{};
};

class TinyNeuralDenoiser {
    TinyNeuralModel model_{};
    bool configured_{false};
public:
    TinyNeuralDenoiser() = default;
    explicit TinyNeuralDenoiser(const TinyNeuralModel& model) : model_(model), configured_(true) {}
    void set_model(const TinyNeuralModel& model) { model_=model; configured_=true; }
    bool configured() const { return configured_; }
    Framebuffer denoise(const Framebuffer& noisy,const Guides& guides) const;
};

// ---------------- Closed-loop frame budget controller ----------------

struct FrameBudgetConfig {
    double target_ms{16.667};
    int min_samples{1};
    int max_samples{64};
    int min_bounces{2};
    int max_bounces{12};
    Scalar min_foveation{0};
    Scalar max_foveation{0.92f};
    int min_history{2};
    int max_history{32};
    double kp{0.35};
    double ki{0.04};
    double kd{0.08};
};

struct FrameBudgetDecision {
    int samples{8};
    int max_bounces{8};
    Scalar foveation_strength{0.5f};
    int reconstruction_history{12};
    double quality_scale{1.0};
};

class FrameBudgetController {
    FrameBudgetConfig config_{};
    double integral_{0};
    double previous_error_{0};
    bool have_previous_{false};
    double quality_{0.5};
public:
    explicit FrameBudgetController(FrameBudgetConfig config={});
    void reset(double quality=0.5);
    FrameBudgetDecision update(double observed_ms);
    void apply(Settings& settings,const FrameBudgetDecision& decision) const;
    const FrameBudgetConfig& config() const { return config_; }
};

// ---------------- Reproducible price/performance metrics ----------------

struct ImageQualityMetrics {
    double mse{0};
    double psnr_db{0};
    double ssim{0};
};

ImageQualityMetrics compare_images(const Framebuffer& image,const Framebuffer& reference);
std::optional<double> read_linux_rapl_joules();
std::size_t process_resident_bytes();

struct PricePerformanceRecord {
    std::string device;
    NativeBackend backend{NativeBackend::CPU};
    double purchase_price_usd{0};
    double frame_ms{0};
    double energy_joules{0};
    std::size_t resident_bytes{0};
    ImageQualityMetrics quality{};
    double fps_per_dollar{0};
    double quality_per_joule{0};
};

PricePerformanceRecord make_price_performance_record(std::string device,
                                                      NativeBackend backend,
                                                      double purchase_price_usd,
                                                      double frame_ms,
                                                      double energy_joules,
                                                      std::size_t resident_bytes,
                                                      const Framebuffer& image,
                                                      const Framebuffer& reference);
std::string price_performance_csv_header();
std::string price_performance_csv_row(const PricePerformanceRecord& record);

} // namespace beamcast::gx
