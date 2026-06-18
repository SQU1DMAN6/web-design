{
  "targets": [
    {
      "target_name": "winfsp_native",
      "sources": ["src/addon.cpp"],
      "include_dirs": [
        "node_modules/node-addon-api",
        "C:\\Program Files (x86)\\WinFsp\\inc\\fuse3",
        "C:\\Program Files (x86)\\WinFsp\\inc"
      ],
      "defines": ["NAPI_VERSION=9"],
      "msvs_settings": {
        "VCCLCompilerTool": {
          "ExceptionHandling": 1
        }
      },
      "link_settings": {
        "libraries": [
          "-lC:\\Program Files (x86)\\WinFsp\\lib\\winfsp-x64.lib"
        ]
      }
    }
  ]
}