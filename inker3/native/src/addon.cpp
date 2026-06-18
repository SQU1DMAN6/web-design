/**
 * WinFsp FUSE3 Native Addon for Inker
 *
 * Architecture:
 *   JS passes InkDrop file entries + content at mount time -> C++ in-memory map -> WinFsp -> Explorer
 *   Content is pre-loaded (base64) for instant file reads. Large files get first 64KB.
 */

#include <napi.h>
#include <fuse.h>
#include <string>
#include <unordered_map>
#include <thread>
#include <mutex>
#include <cstring>
#include <cerrno>
#include <vector>
#include <sstream>
#include <algorithm>
#include <cstdint>
#include <array>

// Simple base64 decode
static std::string base64_decode(const std::string &in) {
    std::string out;
    std::array<int, 256> tbl; tbl.fill(-1);
    for (int i = 0; i < 64; i++) tbl["ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"[i]] = i;
    int val = 0, valb = -8;
    for (unsigned char c : in) {
        if (tbl[c] == -1) continue;
        val = (val << 6) + tbl[c];
        valb += 6;
        if (valb >= 0) { out.push_back((val >> valb) & 0xFF); valb -= 8; }
    }
    return out;
}

#define DEFAULT_BLOCK_SIZE 4096
#define TOTAL_BLOCKS    1048576
#define FREE_BLOCKS     524288

struct FileEntry {
    std::string name;
    long long size;
    long long mtime;
    bool is_dir;
    std::string content; // raw file bytes as string for read_cb
};

struct MountState {
    struct fuse3 *fuse;
    std::thread *loop_thread;
    bool running;
    std::string mountpoint;
    std::unordered_map<std::string, FileEntry> entries;
    MountState() : fuse(nullptr), loop_thread(nullptr), running(false) {}
};

static std::unordered_map<std::string, MountState*> g_mounts;
static std::mutex g_mounts_mutex;

static int getattr_cb(const char *path, struct fuse_stat *stbuf, struct fuse3_file_info *fi) {
    MountState *state = (MountState*)fuse3_get_context()->private_data;
    if (!state || !state->running) return -ENOENT;
    std::memset(stbuf, 0, sizeof(struct fuse_stat));
    if (std::strcmp(path, "/") == 0) {
        stbuf->st_mode = S_IFDIR | 0755; stbuf->st_nlink = 2;
        return 0;
    }
    std::string key(path);
    if (key.size() > 1 && key[0] == '/') key = key.substr(1);
    auto it = state->entries.find(key);
    if (it != state->entries.end()) {
        if (it->second.is_dir) { stbuf->st_mode = S_IFDIR | 0755; stbuf->st_nlink = 2; }
        else { stbuf->st_mode = S_IFREG | 0644; stbuf->st_nlink = 1; stbuf->st_size = it->second.content.size(); }
        stbuf->st_mtim.tv_sec = it->second.mtime;
        stbuf->st_ctim.tv_sec = it->second.mtime;
        stbuf->st_atim.tv_sec = it->second.mtime;
        return 0;
    }
    return -ENOENT;
}

static int readdir_cb(const char *path, void *buf, fuse3_fill_dir_t filler,
                      fuse_off_t off, struct fuse3_file_info *fi,
                      enum fuse3_readdir_flags flags) {
    MountState *state = (MountState*)fuse3_get_context()->private_data;
    if (!state || !state->running) return -ENOENT;
    filler(buf, ".", NULL, 0, (fuse3_fill_dir_flags)0);
    filler(buf, "..", NULL, 0, (fuse3_fill_dir_flags)0);
    std::string dirPath(path);
    if (dirPath == "/") dirPath = "";
    else if (dirPath[0] == '/') dirPath = dirPath.substr(1);
    if (!dirPath.empty() && dirPath.back() != '/') dirPath += "/";
    std::vector<std::pair<std::string, std::string>> children; // <basename, fullkey>
    for (auto &kv : state->entries) {
        if (kv.first.compare(0, dirPath.size(), dirPath) == 0) {
            std::string rest = kv.first.substr(dirPath.size());
            if (rest.find('/') == std::string::npos) {
                children.push_back({rest, kv.first});
            }
        }
    }
    std::sort(children.begin(), children.end(),
        [](auto &a, auto &b) { return a.first < b.first; });
    for (auto &child : children) {
        struct fuse_stat st;
        std::memset(&st, 0, sizeof(st));
        auto it = state->entries.find(child.second);
        if (it != state->entries.end()) {
            if (it->second.is_dir) { st.st_mode = S_IFDIR | 0755; st.st_nlink = 2; }
            else { st.st_mode = S_IFREG | 0644; st.st_nlink = 1; st.st_size = it->second.content.size(); }
        }
        filler(buf, child.first.c_str(), &st, 0, (fuse3_fill_dir_flags)0);
    }
    return 0;
}

static int read_cb(const char *path, char *buf, size_t size, fuse_off_t off, struct fuse3_file_info *fi) {
    MountState *state = (MountState*)fuse3_get_context()->private_data;
    if (!state || !state->running) return -ENOENT;
    std::string key(path);
    if (key.size() > 1 && key[0] == '/') key = key.substr(1);
    auto it = state->entries.find(key);
    if (it == state->entries.end() || it->second.is_dir) return -ENOENT;
    
    const std::string &content = it->second.content;
    if (off >= (fuse_off_t)content.size()) return 0; // EOF
    
    size_t available = content.size() - (size_t)off;
    size_t to_copy = std::min(size, available);
    std::memcpy(buf, content.data() + off, to_copy);
    printf("[FUSE:read] path=%s offset=%lld req=%zu served=%zu\n", path, (long long)off, size, to_copy);
    return (int)to_copy;
}

static int write_cb(const char *path, const char *buf, size_t size, fuse_off_t off, struct fuse3_file_info *fi) { return size; }
static int mkdir_cb(const char *path, fuse_mode_t mode) { return -ENOENT; }
static int unlink_cb(const char *path) { return -ENOENT; }
static int rmdir_cb(const char *path) { return -ENOENT; }
static int rename_cb(const char *oldpath, const char *newpath, unsigned int flags) { return -ENOENT; }
static int create_cb(const char *path, fuse_mode_t mode, struct fuse3_file_info *fi) { fi->fh = 1; return 0; }
static int open_cb(const char *path, struct fuse3_file_info *fi) { fi->fh = 1; return 0; }
static int release_cb(const char *path, struct fuse3_file_info *fi) { return 0; }
static int truncate_cb(const char *path, fuse_off_t size, struct fuse3_file_info *fi) { return 0; }
static int flush_cb(const char *path, struct fuse3_file_info *fi) { return 0; }

static int statfs_cb(const char *path, struct fuse_statvfs *stbuf) {
    std::memset(stbuf, 0, sizeof(struct fuse_statvfs));
    stbuf->f_bsize = DEFAULT_BLOCK_SIZE; stbuf->f_frsize = DEFAULT_BLOCK_SIZE;
    stbuf->f_blocks = TOTAL_BLOCKS; stbuf->f_bfree = FREE_BLOCKS; stbuf->f_bavail = FREE_BLOCKS;
    stbuf->f_files = 1000000; stbuf->f_ffree = 900000; stbuf->f_namemax = 255;
    return 0;
}
static int chmod_cb(const char *path, fuse_mode_t mode, struct fuse3_file_info *fi) { return 0; }
static int utimens_cb(const char *path, const struct fuse_timespec tv[2], struct fuse3_file_info *fi) { return 0; }

static void* init_cb(struct fuse3_conn_info *conn, struct fuse3_config *conf) {
    printf("[FUSE:init] InkDrop Drive mounting...\n");
    conf->entry_timeout = 0; conf->attr_timeout = 0; conf->negative_timeout = 0;
    conf->kernel_cache = 0; conf->auto_cache = 0; conf->direct_io = 1;
    return fuse3_get_context()->private_data;
}
static void destroy_cb(void *data) { printf("[FUSE:destroy] InkDrop Drive unmounted\n"); }

static struct fuse3_operations g_ops = {0};
static bool g_ops_init = false;
static void init_ops() {
    if (g_ops_init) return;
    g_ops.getattr = getattr_cb; g_ops.read = read_cb; g_ops.write = write_cb;
    g_ops.mkdir = mkdir_cb; g_ops.unlink = unlink_cb; g_ops.rmdir = rmdir_cb;
    g_ops.rename = rename_cb; g_ops.chmod = chmod_cb; g_ops.truncate = truncate_cb;
    g_ops.open = open_cb; g_ops.readdir = readdir_cb; g_ops.create = create_cb;
    g_ops.release = release_cb; g_ops.flush = flush_cb; g_ops.statfs = statfs_cb;
    g_ops.init = init_cb; g_ops.destroy = destroy_cb; g_ops.utimens = utimens_cb;
    g_ops_init = true;
}

Napi::Value Mount(const Napi::CallbackInfo &info) {
    Napi::Env env = info.Env();
    if (info.Length() < 2 || !info[0].IsString()) {
        Napi::TypeError::New(env, "Expected: mount(driveLetter, entriesArray)").ThrowAsJavaScriptException();
        return env.Null();
    }

    std::string driveLetter = info[0].As<Napi::String>().Utf8Value();
    std::string mountpoint = driveLetter;
    if (mountpoint.back() != ':') mountpoint += ":";
    if (mountpoint.size() > 1 && mountpoint.back() == '\\') mountpoint.pop_back();
    printf("[NATIVE:mount] Mounting at %s\n", mountpoint.c_str());

    MountState *state = new MountState();
    state->mountpoint = mountpoint;
    state->running = true;

    if (info[1].IsArray()) {
        Napi::Array entries = info[1].As<Napi::Array>();
        uint32_t len = entries.Length();
        size_t totalContentBytes = 0;
        size_t fileCount = 0;
        printf("[NATIVE:mount] %u entries from InkDrop\n", len);
        for (uint32_t i = 0; i < len; i++) {
            Napi::Value val = entries.Get(i);
            if (!val.IsObject()) continue;
            Napi::Object obj = val.As<Napi::Object>();
            FileEntry fe;
            if (obj.Has("path") && obj.Get("path").IsString())
                fe.name = obj.Get("path").As<Napi::String>().Utf8Value();
            else continue;
            fe.size = obj.Has("size") ? obj.Get("size").As<Napi::Number>().Int64Value() : 0;
            fe.mtime = obj.Has("modified") ? static_cast<long long>(obj.Get("modified").As<Napi::Number>().DoubleValue()) : 0;
            if (obj.Has("type")) { std::string t = obj.Get("type").As<Napi::String>().Utf8Value(); fe.is_dir = (t == "dir"); }
            
            // Read content (base64-encoded from JS, decode to raw bytes)
            if (obj.Has("content") && obj.Get("content").IsString() && !fe.is_dir) {
                std::string b64 = obj.Get("content").As<Napi::String>().Utf8Value();
                fe.content = base64_decode(b64);
                totalContentBytes += fe.content.size();
                fileCount++;
            }
            
            state->entries[fe.name] = fe;
            // Auto-create parent directories
            size_t pos = 0;
            while ((pos = fe.name.find('/', pos)) != std::string::npos) {
                std::string dirPath = fe.name.substr(0, pos);
                if (state->entries.find(dirPath) == state->entries.end()) {
                    FileEntry dirFe; dirFe.name = dirPath; dirFe.size = 0; dirFe.mtime = fe.mtime; dirFe.is_dir = true;
                    state->entries[dirPath] = dirFe;
                }
                pos++;
            }
        }
        printf("[NATIVE:mount] %zu entries in C++ map, %zu files with content (%zu bytes)\n",
               state->entries.size(), fileCount, totalContentBytes);
    }

    init_ops();
    { std::lock_guard<std::mutex> lock(g_mounts_mutex); g_mounts[mountpoint] = state; }

    state->loop_thread = new std::thread([state, mountpoint]() {
        printf("[NATIVE:thread] FUSE loop starting...\n");
        const char *fuse_argv[] = {"inker", mountpoint.c_str(), "-s",
            "-o", "volname=InkDrop Drive",
            "-o", "umask=000",
            "-o", "uid=-1", "-o", "gid=-1",
            NULL};
        int fuse_argc = 9;
        struct fuse_args args = FUSE_ARGS_INIT(fuse_argc, (char**)fuse_argv);
        struct fuse3 *fuse = fuse3_new(&args, &g_ops, sizeof(g_ops), state);
        if (!fuse) { printf("[NATIVE:thread] fuse3_new FAILED\n"); state->running = false; return; }
        state->fuse = fuse;
        int ret = fuse3_mount(fuse, mountpoint.c_str());
        if (ret != 0) { fuse3_destroy(fuse); state->running = false; return; }
        printf("[NATIVE:thread] === Q:\\ IS LIVE with %zu entries (content: %zu bytes) ===\n", state->entries.size(), state->entries.size() > 0 ? state->entries.begin()->second.content.size() : 0);
        fuse3_loop(fuse);
        fuse3_unmount(fuse); fuse3_destroy(fuse); state->running = false;
    });
    state->loop_thread->detach();
    return env.Undefined();
}

Napi::Value Unmount(const Napi::CallbackInfo &info) {
    Napi::Env env = info.Env();
    if (info.Length() < 1 || !info[0].IsString()) { Napi::TypeError::New(env, "Expected: unmount(driveLetter)").ThrowAsJavaScriptException(); return env.Null(); }
    std::string dl = info[0].As<Napi::String>().Utf8Value();
    if (dl.back() != ':') dl += ":";
    if (dl.size() > 1 && dl.back() == '\\') dl.pop_back();
    printf("[NATIVE:unmount] %s\n", dl.c_str());
    MountState *state = nullptr;
    { std::lock_guard<std::mutex> lock(g_mounts_mutex); auto it = g_mounts.find(dl); if (it != g_mounts.end()) { state = it->second; g_mounts.erase(it); } }
    if (!state || !state->fuse) { return env.Undefined(); }
    fuse3_exit(state->fuse);
    if (state->loop_thread && state->loop_thread->joinable()) state->loop_thread->join();
    delete state;
    printf("[NATIVE:unmount] Complete\n");
    return env.Undefined();
}

Napi::Value IsAvailable(const Napi::CallbackInfo &info) { return Napi::Boolean::New(info.Env(), true); }
Napi::Value GetVersion(const Napi::CallbackInfo &info) { return Napi::String::New(info.Env(), "Inker WinFsp FUSE3 v1.0"); }

Napi::Object Init(Napi::Env env, Napi::Object exports) {
    exports.Set("mount", Napi::Function::New(env, Mount));
    exports.Set("unmount", Napi::Function::New(env, Unmount));
    exports.Set("isAvailable", Napi::Function::New(env, IsAvailable));
    exports.Set("getVersion", Napi::Function::New(env, GetVersion));
    printf("[WIN] Inker native addon loaded\n");
    return exports;
}

NODE_API_MODULE(winfsp_native, Init)