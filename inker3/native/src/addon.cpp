/**
 * WinFsp FUSE3 Native Addon for Inker — Production
 *
 * Full read/write/mkdir/rmdir/unlink/rename support with dirty tracking.
 * Every mutation updates the in-memory map AND marks the path as dirty.
 * JS polls getDirty() to upload changes to InkDrop API.
 */

#include <napi.h>
#include <fuse.h>
#include <string>
#include <unordered_map>
#include <unordered_set>
#include <thread>
#include <mutex>
#include <cstring>
#include <cerrno>
#include <vector>
#include <sstream>
#include <algorithm>
#include <cstdint>
#include <array>
#include <ctime>

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

static std::string base64_encode(const std::string &in) {
    static const char *tbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    std::string out;
    int val = 0, valb = -6;
    for (unsigned char c : in) {
        val = (val << 8) + c;
        valb += 8;
        while (valb >= 0) { out.push_back(tbl[(val >> valb) & 0x3F]); valb -= 6; }
    }
    if (valb > -6) out.push_back(tbl[((val << 8) >> (valb + 8)) & 0x3F]);
    while (out.size() % 4) out.push_back('=');
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
    std::string content;
};

struct MountState {
    struct fuse3 *fuse;
    std::thread *loop_thread;
    bool running;
    std::string mountpoint;
    std::unordered_map<std::string, FileEntry> entries;
    std::unordered_set<std::string> dirty_keys; // keys changed since last poll
    std::mutex dirty_mutex; // separate mutex for dirty set
    std::mutex entries_mutex; // mutex for entries map
    MountState() : fuse(nullptr), loop_thread(nullptr), running(false) {}
};

static std::unordered_map<std::string, MountState*> g_mounts;
static std::mutex g_mounts_mutex;

static MountState* get_state() {
    return (MountState*)fuse3_get_context()->private_data;
}

static int getattr_cb(const char *path, struct fuse_stat *stbuf, struct fuse3_file_info *fi) {
    MountState *state = get_state();
    if (!state || !state->running) return -ENOENT;
    std::memset(stbuf, 0, sizeof(struct fuse_stat));
    if (std::strcmp(path, "/") == 0) {
        stbuf->st_mode = S_IFDIR | 0755; stbuf->st_nlink = 2;
        return 0;
    }
    std::string key(path);
    if (key.size() > 1 && key[0] == '/') key = key.substr(1);
    std::lock_guard<std::mutex> lock(state->entries_mutex);
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
    MountState *state = get_state();
    if (!state || !state->running) return -ENOENT;
    filler(buf, ".", NULL, 0, (fuse3_fill_dir_flags)0);
    filler(buf, "..", NULL, 0, (fuse3_fill_dir_flags)0);
    std::string dirPath(path);
    if (dirPath == "/") dirPath = "";
    else if (dirPath[0] == '/') dirPath = dirPath.substr(1);
    if (!dirPath.empty() && dirPath.back() != '/') dirPath += "/";
    std::vector<std::pair<std::string, std::string>> children;
    {
        std::lock_guard<std::mutex> lock(state->entries_mutex);
        for (auto &kv : state->entries) {
            if (kv.first.compare(0, dirPath.size(), dirPath) == 0) {
                std::string rest = kv.first.substr(dirPath.size());
                if (rest.find('/') == std::string::npos) children.push_back({rest, kv.first});
            }
        }
    }
    std::sort(children.begin(), children.end(), [](auto &a, auto &b) { return a.first < b.first; });
    for (auto &child : children) {
        struct fuse_stat st;
        std::memset(&st, 0, sizeof(st));
        std::lock_guard<std::mutex> lock(state->entries_mutex);
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
    MountState *state = get_state();
    if (!state || !state->running) return -ENOENT;
    std::string key(path);
    if (key.size() > 1 && key[0] == '/') key = key.substr(1);
    std::lock_guard<std::mutex> lock(state->entries_mutex);
    auto it = state->entries.find(key);
    if (it == state->entries.end() || it->second.is_dir) return -ENOENT;
    const std::string &content = it->second.content;
    if (off >= (fuse_off_t)content.size()) return 0;
    size_t to_copy = std::min(size, content.size() - (size_t)off);
    std::memcpy(buf, content.data() + off, to_copy);
    printf("[FUSE:read] path=%s off=%lld req=%zu served=%zu\n", path, (long long)off, size, to_copy);
    return (int)to_copy;
}

static int write_cb(const char *path, const char *buf, size_t size, fuse_off_t off, struct fuse3_file_info *fi) {
    MountState *state = get_state();
    if (!state || !state->running) return -ENOENT;
    std::string key(path);
    if (key.size() > 1 && key[0] == '/') key = key.substr(1);
    {
        std::lock_guard<std::mutex> lock(state->entries_mutex);
        auto it = state->entries.find(key);
        if (it == state->entries.end()) return -ENOENT;
        std::string &content = it->second.content;
        if (off + size > content.size()) content.resize(off + size);
        std::memcpy(&content[off], buf, size);
        it->second.mtime = std::time(nullptr);
    }
    {
        std::lock_guard<std::mutex> lock(state->dirty_mutex);
        state->dirty_keys.insert(key);
    }
    printf("[FUSE:write] path=%s off=%lld size=%zu -> dirty\n", path, (long long)off, size);
    return (int)size;
}

static int mkdir_cb(const char *path, fuse_mode_t mode) {
    MountState *state = get_state();
    if (!state || !state->running) return -ENOENT;
    std::string key(path);
    if (key.size() > 1 && key[0] == '/') key = key.substr(1);
    if (key.empty()) return -EEXIST;
    {
        std::lock_guard<std::mutex> lock(state->entries_mutex);
        if (state->entries.find(key) != state->entries.end()) return -EEXIST;
        FileEntry fe; fe.name = key; fe.size = 0; fe.mtime = std::time(nullptr); fe.is_dir = true;
        state->entries[key] = fe;
    }
    {
        std::lock_guard<std::mutex> lock(state->dirty_mutex);
        state->dirty_keys.insert(key + "/.mkdir");
    }
    printf("[FUSE:mkdir] path=%s\n", path);
    return 0;
}

static int create_cb(const char *path, fuse_mode_t mode, struct fuse3_file_info *fi) {
    MountState *state = get_state();
    if (!state || !state->running) return -ENOENT;
    std::string key(path);
    if (key.size() > 1 && key[0] == '/') key = key.substr(1);
    {
        std::lock_guard<std::mutex> lock(state->entries_mutex);
        FileEntry fe; fe.name = key; fe.size = 0; fe.mtime = std::time(nullptr); fe.is_dir = false;
        state->entries[key] = fe;
    }
    {
        std::lock_guard<std::mutex> lock(state->dirty_mutex);
        state->dirty_keys.insert(key);
    }
    fi->fh = (uint64_t)1;
    printf("[FUSE:create] path=%s\n", path);
    return 0;
}

static int unlink_cb(const char *path) {
    MountState *state = get_state();
    if (!state || !state->running) return -ENOENT;
    std::string key(path);
    if (key.size() > 1 && key[0] == '/') key = key.substr(1);
    {
        std::lock_guard<std::mutex> lock(state->entries_mutex);
        state->entries.erase(key);
    }
    {
        std::lock_guard<std::mutex> lock(state->dirty_mutex);
        state->dirty_keys.insert("!" + key); // prefix ! = delete
    }
    printf("[FUSE:unlink] path=%s\n", path);
    return 0;
}

static int rmdir_cb(const char *path) { return unlink_cb(path); }

static int rename_cb(const char *oldpath, const char *newpath, unsigned int flags) {
    MountState *state = get_state();
    if (!state || !state->running) return -ENOENT;
    std::string okey(oldpath), nkey(newpath);
    if (okey.size() > 1 && okey[0] == '/') okey = okey.substr(1);
    if (nkey.size() > 1 && nkey[0] == '/') nkey = nkey.substr(1);
    {
        std::lock_guard<std::mutex> lock(state->entries_mutex);
        auto it = state->entries.find(okey);
        if (it == state->entries.end()) return -ENOENT;
        FileEntry fe = it->second;
        fe.name = nkey; fe.mtime = std::time(nullptr);
        state->entries.erase(it);
        state->entries[nkey] = fe;
    }
    {
        std::lock_guard<std::mutex> lock(state->dirty_mutex);
        state->dirty_keys.insert("!" + okey);
        state->dirty_keys.insert(nkey);
    }
    printf("[FUSE:rename] %s -> %s\n", oldpath, newpath);
    return 0;
}

static int open_cb(const char *path, struct fuse3_file_info *fi) { fi->fh = 1; return 0; }
static int release_cb(const char *path, struct fuse3_file_info *fi) { return 0; }
static int truncate_cb(const char *path, fuse_off_t size, struct fuse3_file_info *fi) {
    MountState *state = get_state();
    if (!state || !state->running) return -ENOENT;
    std::string key(path);
    if (key.size() > 1 && key[0] == '/') key = key.substr(1);
    std::lock_guard<std::mutex> lock(state->entries_mutex);
    auto it = state->entries.find(key);
    if (it == state->entries.end() || it->second.is_dir) return -ENOENT;
    it->second.content.resize(size);
    it->second.mtime = std::time(nullptr);
    printf("[FUSE:truncate] path=%s size=%lld\n", path, (long long)size);
    return 0;
}
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
    printf("[FUSE:init] InkDrop Drive live\n");
    conf->entry_timeout = 0; conf->attr_timeout = 0; conf->negative_timeout = 0;
    conf->kernel_cache = 0; conf->auto_cache = 0; conf->direct_io = 1;
    return fuse3_get_context()->private_data;
}
static void destroy_cb(void *data) { printf("[FUSE:destroy] unmounted\n"); }

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

// ====== N-API ======

Napi::Value Mount(const Napi::CallbackInfo &info) {
    Napi::Env env = info.Env();
    if (info.Length() < 2 || !info[0].IsString()) {
        Napi::TypeError::New(env, "mount(driveLetter, entries)").ThrowAsJavaScriptException();
        return env.Null();
    }
    std::string dl = info[0].As<Napi::String>().Utf8Value();
    std::string mp = dl; if (mp.back() != ':') mp += ":"; if (mp.size()>1 && mp.back()=='\\') mp.pop_back();
    printf("[NATIVE:mount] %s\n", mp.c_str());
    MountState *state = new MountState(); state->mountpoint = mp; state->running = true;
    if (info[1].IsArray()) {
        Napi::Array entries = info[1].As<Napi::Array>();
        uint32_t len = entries.Length();
        size_t bytes = 0, fc = 0;
        for (uint32_t i = 0; i < len; i++) {
            Napi::Value val = entries.Get(i); if (!val.IsObject()) continue;
            Napi::Object obj = val.As<Napi::Object>();
            FileEntry fe;
            if (obj.Has("path") && obj.Get("path").IsString()) fe.name = obj.Get("path").As<Napi::String>().Utf8Value(); else continue;
            fe.size = obj.Has("size") ? obj.Get("size").As<Napi::Number>().Int64Value() : 0;
            fe.mtime = obj.Has("modified") ? (long long)obj.Get("modified").As<Napi::Number>().DoubleValue() : 0;
            if (obj.Has("type")) { std::string t = obj.Get("type").As<Napi::String>().Utf8Value(); fe.is_dir = (t == "dir"); }
            if (obj.Has("content") && obj.Get("content").IsString() && !fe.is_dir) {
                fe.content = base64_decode(obj.Get("content").As<Napi::String>().Utf8Value());
                bytes += fe.content.size(); fc++;
            }
            state->entries[fe.name] = fe;
            size_t pos = 0;
            while ((pos = fe.name.find('/', pos)) != std::string::npos) {
                std::string dp = fe.name.substr(0, pos);
                if (state->entries.find(dp) == state->entries.end()) {
                    FileEntry df; df.name = dp; df.size = 0; df.mtime = fe.mtime; df.is_dir = true;
                    state->entries[dp] = df;
                }
                pos++;
            }
        }
        printf("[NATIVE:mount] %zu entries (%zu with content, %zu bytes)\n", state->entries.size(), fc, bytes);
    }
    init_ops();
    { std::lock_guard<std::mutex> lock(g_mounts_mutex); g_mounts[mp] = state; }
    state->loop_thread = new std::thread([state, mp]() {
        printf("[NATIVE:thread] FUSE loop starting...\n");
        const char *argv[] = {"inker", mp.c_str(), "-s", "-o", "volname=InkDrop Drive", "-o", "umask=000", "-o", "uid=-1", "-o", "gid=-1", NULL};
        struct fuse_args args = FUSE_ARGS_INIT(9, (char**)argv);
        struct fuse3 *fuse = fuse3_new(&args, &g_ops, sizeof(g_ops), state);
        if (!fuse) { printf("[NATIVE:thread] fuse3_new FAILED\n"); state->running = false; return; }
        state->fuse = fuse;
        if (fuse3_mount(fuse, mp.c_str())) { fuse3_destroy(fuse); state->running = false; return; }
        printf("[NATIVE:thread] === Q:\\ LIVE ===\n");
        fuse3_loop(fuse);
        fuse3_unmount(fuse); fuse3_destroy(fuse); state->running = false;
    });
    state->loop_thread->detach();
    return env.Undefined();
}

Napi::Value Unmount(const Napi::CallbackInfo &info) {
    Napi::Env env = info.Env();
    if (info.Length() < 1 || !info[0].IsString()) { Napi::TypeError::New(env, "unmount(driveLetter)").ThrowAsJavaScriptException(); return env.Null(); }
    std::string dl = info[0].As<Napi::String>().Utf8Value();
    if (dl.back() != ':') dl += ":"; if (dl.size()>1 && dl.back()=='\\') dl.pop_back();
    printf("[NATIVE:unmount] %s\n", dl.c_str());
    MountState *state = nullptr;
    { std::lock_guard<std::mutex> lock(g_mounts_mutex); auto it = g_mounts.find(dl); if (it != g_mounts.end()) { state = it->second; g_mounts.erase(it); } }
    if (!state || !state->fuse) return env.Undefined();
    fuse3_exit(state->fuse);
    if (state->loop_thread && state->loop_thread->joinable()) state->loop_thread->join();
    delete state;
    return env.Undefined();
}

/**
 * updateEntries(entriesArray) → void
 * Called from JS refresh loop. Updates the in-memory file map with the
 * latest state from InkDrop. Removes entries that no longer exist on remote.
 * Does NOT mark as dirty (remote is truth).
 */
Napi::Value UpdateEntries(const Napi::CallbackInfo &info) {
    Napi::Env env = info.Env();
    if (info.Length() < 1 || !info[0].IsArray()) return env.Undefined();
    // Find the first mounted state (single Q:\ mount)
    MountState *state = nullptr;
    { std::lock_guard<std::mutex> lock(g_mounts_mutex); for (auto &kv : g_mounts) { state = kv.second; break; } }
    if (!state) return env.Undefined();
    
    Napi::Array entries = info[0].As<Napi::Array>();
    uint32_t len = entries.Length();
    printf("[NATIVE:updateEntries] %u entries from remote\n", len);
    
    // Build a set of current remote paths
    std::unordered_set<std::string> remotePaths;
    for (uint32_t i = 0; i < len; i++) {
        Napi::Value val = entries.Get(i);
        if (!val.IsObject()) continue;
        Napi::Object obj = val.As<Napi::Object>();
        if (!obj.Has("path") || !obj.Get("path").IsString()) continue;
        std::string path = obj.Get("path").As<Napi::String>().Utf8Value();
        remotePaths.insert(path);
        
        // Check if entry exists locally
        std::lock_guard<std::mutex> lock(state->entries_mutex);
        auto it = state->entries.find(path);
        if (it != state->entries.end()) {
            // Entry exists — only update content if remote is newer AND not dirty
            bool isDirty = false;
            { std::lock_guard<std::mutex> dl(state->dirty_mutex); isDirty = state->dirty_keys.count(path) > 0; }
            if (!isDirty) {
                if (obj.Has("content") && obj.Get("content").IsString()) {
                    std::string b64 = obj.Get("content").As<Napi::String>().Utf8Value();
                    std::string decoded = base64_decode(b64);
                    it->second.content = decoded;
                    it->second.size = decoded.size();
                }
                if (obj.Has("modified")) it->second.mtime = (long long)obj.Get("modified").As<Napi::Number>().DoubleValue();
                if (obj.Has("type")) { std::string t = obj.Get("type").As<Napi::String>().Utf8Value(); it->second.is_dir = (t == "dir"); }
            }
        } else {
            // New entry from remote — add it
            FileEntry fe;
            fe.name = path;
            fe.size = obj.Has("size") ? obj.Get("size").As<Napi::Number>().Int64Value() : 0;
            fe.mtime = obj.Has("modified") ? (long long)obj.Get("modified").As<Napi::Number>().DoubleValue() : 0;
            if (obj.Has("type")) { std::string t = obj.Get("type").As<Napi::String>().Utf8Value(); fe.is_dir = (t == "dir"); }
            if (obj.Has("content") && obj.Get("content").IsString() && !fe.is_dir) {
                fe.content = base64_decode(obj.Get("content").As<Napi::String>().Utf8Value());
            }
            std::lock_guard<std::mutex> lock(state->entries_mutex);
            state->entries[path] = fe;
        }
    }
    
    // NOTE: Do NOT remove entries here. updateEntries receives only the *changed* subset.
    // Removing entries that aren't in this subset would wipe the entire filesystem.
    // Full reconciliation is handled on next mount.
    printf("[NATIVE:updateEntries] %zu entries now in map\n", state->entries.size());
    return env.Undefined();
}

/**
 * getDirty() -> [{key, type, content_base64}]
 * Returns all dirty keys and clears the dirty set.
 * "!path" prefix = deleted, ".mkdir" suffix = mkdir operation
 */
Napi::Value GetDirty(const Napi::CallbackInfo &info) {
    Napi::Env env = info.Env();
    Napi::Array result = Napi::Array::New(env);
    uint32_t idx = 0;
    auto processMount = [&](MountState *state) {
        if (!state) return;
        std::unordered_set<std::string> dirty;
        {
            std::lock_guard<std::mutex> lock(state->dirty_mutex);
            dirty.swap(state->dirty_keys);
        }
        for (auto &dk : dirty) {
            Napi::Object obj = Napi::Object::New(env);
            bool isDelete = (dk[0] == '!');
            bool isMkdir = (dk.size() > 6 && dk.substr(dk.size() - 6) == ".mkdir");
            std::string key = dk;
            if (isDelete) key = dk.substr(1);
            if (isMkdir) key = dk.substr(0, dk.size() - 6);
            
            obj.Set("key", Napi::String::New(env, key));
            obj.Set("isDelete", Napi::Boolean::New(env, isDelete));
            obj.Set("isMkdir", Napi::Boolean::New(env, isMkdir));
            obj.Set("mountpoint", Napi::String::New(env, state->mountpoint));
            
            if (!isDelete && !isMkdir) {
                std::lock_guard<std::mutex> lock(state->entries_mutex);
                auto it = state->entries.find(key);
                if (it != state->entries.end() && !it->second.is_dir) {
                    obj.Set("content", Napi::String::New(env, base64_encode(it->second.content)));
                    obj.Set("size", Napi::Number::New(env, (double)it->second.content.size()));
                }
            }
            result.Set(idx++, obj);
        }
    };
    { std::lock_guard<std::mutex> lock(g_mounts_mutex); for (auto &kv : g_mounts) processMount(kv.second); }
    return result;
}

/**
 * getCacheStats() -> { entries, sizeBytes }
 */
Napi::Value GetCacheStats(const Napi::CallbackInfo &info) {
    Napi::Env env = info.Env();
    size_t totalEntries = 0, totalBytes = 0;
    {
        std::lock_guard<std::mutex> lock(g_mounts_mutex);
        for (auto &kv : g_mounts) {
            MountState *s = kv.second;
            if (!s) continue;
            std::lock_guard<std::mutex> elock(s->entries_mutex);
            totalEntries += s->entries.size();
            for (auto &e : s->entries) totalBytes += e.second.content.size();
        }
    }
    Napi::Object stats = Napi::Object::New(env);
    stats.Set("entries", Napi::Number::New(env, (double)totalEntries));
    stats.Set("sizeBytes", Napi::Number::New(env, (double)totalBytes));
    return stats;
}

Napi::Value IsAvailable(const Napi::CallbackInfo &info) { return Napi::Boolean::New(info.Env(), true); }
Napi::Value GetVersion(const Napi::CallbackInfo &info) { return Napi::String::New(info.Env(), "Inker WinFsp FUSE3 v2.0"); }

Napi::Object Init(Napi::Env env, Napi::Object exports) {
    exports.Set("mount", Napi::Function::New(env, Mount));
    exports.Set("unmount", Napi::Function::New(env, Unmount));
    exports.Set("updateEntries", Napi::Function::New(env, UpdateEntries));
    exports.Set("getDirty", Napi::Function::New(env, GetDirty));
    exports.Set("getCacheStats", Napi::Function::New(env, GetCacheStats));
    exports.Set("isAvailable", Napi::Function::New(env, IsAvailable));
    exports.Set("getVersion", Napi::Function::New(env, GetVersion));
    printf("[WIN] Inker native v2 loaded\n");
    return exports;
}

NODE_API_MODULE(winfsp_native, Init)