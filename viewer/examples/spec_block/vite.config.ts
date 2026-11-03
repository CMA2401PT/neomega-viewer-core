import { defineConfig } from "vite";

export default defineConfig({
  // 本地绘制包和示例必须共用同一份 Three.js，避免打包两套运行时。
  resolve: { dedupe: ["three"] },
});
