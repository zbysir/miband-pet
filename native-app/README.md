# 原生写法的 Agent 猫（对照实验）

用快应用自己的 `.ux` 把 Agent 猫手写一遍（猫页 + 今日页），和插件渲染器比速度。结果见根目录 README「为什么这样写插件」和 [docs/render-spike.md](../docs/render-spike.md)。

- 单独一个应用（包名 `dev.bysir.mibandnative`，手环上叫「猫·原生」），可以和 wristkit 并存。
- 不接 Mac，数据是 `src/common/demo.js` 里编的示例。点击耗时会通过 interconnect 发出去，命令行版 `miband -v app` 的日志里能看到（`原生 点气泡 …ms`）。
- 猫的动画同样是定时换图（小米手环 9 Pro 上拆掉 `image-animator` 会让手环重启）。

构建和安装：

```sh
ln -s ../render-app/node_modules node_modules   # 和渲染器共用依赖（或者 npm install）
npx aiot build
../server/miband install dist/dev.bysir.mibandnative.debug.1.0.0.rpk
```
