/**
 * WinFsp Native Addon Loader
 * Loads the compiled .node addon and provides a clean JS API.
 */
const path = require("path");

let native = null;
try {
    native = require("./build/Release/winfsp_native.node");
    console.log("[WinFspNative] Addon loaded successfully");
    console.log("[WinFspNative] Available:", JSON.stringify(Object.keys(native)));
} catch (err) {
    console.log("[WinFspNative] Failed to load native addon:", err.message);
}

module.exports = native;