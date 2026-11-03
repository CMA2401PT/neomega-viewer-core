import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import {
  createEffectRegistry,
  enchantingTableBookTextureURL,
  createFrameObject,
  decodeFrame,
  loadAtlas,
  type FrameData,
  type FrameObject,
} from "@neomega/viewer/render";

async function main(): Promise<void> {
  // 1. 网格中的 UV 指向这张配套图集；贴图与 Go 使用的几何表来自同一份资源。
  const atlas: THREE.Texture = await loadAtlas("/textures/atlas.png");
  // 书本贴图随前端包交付，URL 由打包器生成；无需在 Go 注册资源路由。
  const book: THREE.Texture = await loadAtlas(enchantingTableBookTextureURL);
  // 2. 请求 Go 生成的二进制结果，解码后创建可以加入 Three.js 场景的对象。
  const response: Response = await fetch("/api/subchunk?subchunk=default");
  if (!response.ok) throw new Error(await response.text());
  // 完整快照包含静态网格、陶罐 actor 网格和书本 effect 消息。
  const binary: ArrayBuffer = await response.arrayBuffer();
  const frameData: FrameData = decodeFrame(binary);
  // registry 把 Go 发来的 effect 名称映射到书本绘制器；actor 已经是网格。
  const frame: FrameObject = createFrameObject(frameData, atlas, createEffectRegistry(book));

  // 3. 以下相机、背景和鼠标控制只决定本示例的取景。
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0xeeeeee);
  scene.add(frame.object);

  const renderer = new THREE.WebGLRenderer({ antialias: true });
  document.body.append(renderer.domElement);
  const camera = new THREE.PerspectiveCamera(45, 1, 0.1, 100);
  camera.position.set(25, 20, 29);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.target.set(8, 4, 8);
  controls.update();

  function resize(): void {
    renderer.setSize(window.innerWidth, window.innerHeight);
    camera.aspect = window.innerWidth / window.innerHeight;
    camera.updateProjectionMatrix();
  }
  window.addEventListener("resize", resize);
  resize();
  // 4. 书本持续起伏、旋转和翻页；回调给出毫秒，updateTime 接收秒。
  renderer.setAnimationLoop((time: number) => {
    frame.updateTime(time / 1000);
    renderer.render(scene, camera);
  });
}

main().catch((error: unknown) => {
  console.error(error);
  document.body.textContent = String(error);
});
