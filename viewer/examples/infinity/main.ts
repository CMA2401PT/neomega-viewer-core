import * as THREE from "three";
import {
  AxisGizmo,
  SectionScene,
  createEffectRegistry,
  loadAtlas,
} from "@neomega/viewer/render";
import { FlightControls } from "./flight.ts";
import { SECTION_SIZE, SectionLoader } from "./stream.ts";

async function main(): Promise<void> {
  // 1. 网格中的 UV 指向这张配套图集；贴图与 Go 使用的几何表来自同一份资源。
  const atlas: THREE.Texture = await loadAtlas("/textures/atlas.png");
  // 书本是独立的动态几何，使用自己的贴图。
  const book: THREE.Texture = await loadAtlas("/textures/entity/enchanting_table_book.png");
  // 2. 设置自由飞行视角；雾在加载范围边缘变浓，避免远处区域突然出现。
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0xd9e5ed);
  scene.fog = new THREE.Fog(0xd9e5ed, 32, 48);
  const renderer = new THREE.WebGLRenderer({ antialias: true });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  document.body.append(renderer.domElement);
  const camera = new THREE.PerspectiveCamera(65, 1, 0.1, 96);
  camera.position.set(8, 15, 24);
  const controls = new FlightControls(camera, renderer.domElement);
  const axes = new AxisGizmo(document.querySelector<HTMLCanvasElement>("#axes")!);
  const position: HTMLOutputElement = document.querySelector<HTMLOutputElement>("#position")!;

  // 3. SectionScene 负责合批、裁剪和书本动画，SectionLoader 只负责请求和卸载。
  // 尺寸由应用固定约定，与 world.go 一致；每份 NVFR 快照无需携带尺寸。
  const world = new SectionScene(
    scene,
    atlas,
    createEffectRegistry(book),
    renderer,
    SECTION_SIZE,
  );
  const loader = new SectionLoader(world);
  function resize(): void {
    renderer.setSize(window.innerWidth, window.innerHeight);
    camera.aspect = window.innerWidth / window.innerHeight;
    camera.updateProjectionMatrix();
  }
  window.addEventListener("resize", resize);
  resize();
  // 4. 移动使用帧间隔，动画使用累计秒数；文字和坐标轴每 100 毫秒更新一次。
  let previousTime: number = performance.now();
  let lastHUD: number = 0;
  renderer.setAnimationLoop((time: number) => {
    // 切回标签页时不把暂停的时间算成一次大幅移动。
    controls.update(Math.min((time - previousTime) / 1000, 0.05));
    previousTime = time;
    loader.update(camera.position);
    world.update(camera, time / 1000);
    world.render(renderer, camera);
    if (time - lastHUD >= 100) {
      axes.update(camera.quaternion);
      position.textContent = loader.error || [
        `X ${camera.position.x.toFixed(1)}`,
        `Y ${camera.position.y.toFixed(1)}`,
        `Z ${camera.position.z.toFixed(1)}`,
      ].join("\n");
      lastHUD = time;
    }
  });
  window.addEventListener("pagehide", () => {
    // 先停止更新和在途请求，再释放区域、共享材质及调用方持有的贴图。
    renderer.setAnimationLoop(null);
    loader.dispose();
    world.dispose();
    axes.dispose();
    atlas.dispose();
    book.dispose();
    renderer.dispose();
  }, { once: true });
}

main().catch((error: unknown) => {
  console.error(error);
  document.querySelector<HTMLOutputElement>("#position")!.textContent = String(error);
});
