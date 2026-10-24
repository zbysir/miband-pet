#!/bin/sh
# 用小米的 aiot-toolkit 把 server/internal/ux/testdata 下的每个 .ux 编一遍，编出来的页面 JS 存成 <名字>.aiot.js，
# 给 Go 写的 .ux 编译器当标准答案（server/internal/ux 的测试拿两边的结果在 goja 里跑，比较画出来的东西）。
# 要先装好 aiot-toolkit（cd tools/ux-oracle && npm install）。只有改了测试用的 .ux 才要重新跑。
set -e
root=$(cd "$(dirname "$0")/.." && pwd)
data="$root/server/internal/ux/testdata"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
for f in "$data"/*.ux; do
  name=$(basename "$f" .ux)
  p="$work/$name"
  mkdir -p "$p/src/pages/index" "$p/src/common"
  cp "$f" "$p/src/pages/index/index.ux"
  cp "$root/server/internal/bandapp/logo.png" "$p/src/common/logo.png"
  [ -d "$data/$name.assets" ] && cp -R "$data/$name.assets/." "$p/src/"
  mkdir -p "$p/sign/debug" && cp "$root"/server/internal/quickapp/sign/*.pem "$p/sign/debug/"
  ln -s "$root/tools/ux-oracle/node_modules" "$p/node_modules"
  printf '<script>\nexport default {}\n</script>\n' > "$p/src/app.ux"
  cat > "$p/src/manifest.json" <<JSON
{"package":"dev.wristkit.oracle","name":"oracle","versionName":"1.0.0","versionCode":1,"minPlatformVersion":1200,
 "icon":"/common/logo.png","deviceTypeList":["watch"],"features":[],"config":{"logLevel":"log","designWidth":212},
 "router":{"entry":"pages/index","pages":{"pages/index":{"component":"index"}}},"minAPILevel":2}
JSON
  printf '{"name":"oracle","version":"1.0.0","private":true}\n' > "$p/package.json"
  (cd "$p" && npx aiot build --complete-feature >"$work/$name.log" 2>&1) || { cat "$work/$name.log"; exit 1; }
  cp "$p/build/pages/index/index.js" "$data/$name.aiot.js"
  echo "$name ok"
done
