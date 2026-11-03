import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import {
  createEffectRegistry,
  createFrameObject,
  decodeFrame,
  loadAtlas,
  type FrameData,
  type FrameObject,
} from "@neomega/viewer/render";

async function main(): Promise<void> {
  const sceneName: string = new URLSearchParams(location.search).get("scene") ?? "waterlogged_leaves";
  // 1. 网格中的 UV 指向这张配套图集；贴图与 Go 使用的几何表来自同一份资源。
  const atlas: THREE.Texture = await loadAtlas("/textures/atlas.png");
  // 书本是独立的动态几何，使用自己的贴图。
  const book: THREE.Texture = await loadAtlas("/textures/entity/enchanting_table_book.png");
  // 2. 请求 Go 生成的二进制结果，解码后创建可以加入 Three.js 场景的对象。
  const response: Response = await fetch(`/api/block?scene=${encodeURIComponent(sceneName)}`);
  if (!response.ok) throw new Error(await response.text());
  // 四种场景共用同一条解码和绘制链路，网页无需区分方块类型。
  const binary: ArrayBuffer = await response.arrayBuffer();
  const frameData: FrameData = decodeFrame(binary);
  // registry 把 Go 发来的 effect 名称映射到书本绘制器；actor 已经是网格。
  const frame: FrameObject = createFrameObject(frameData, atlas, createEffectRegistry(book));

  // 3. 以下相机、背景和鼠标控制只决定本示例的取景。
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0xeeeeee);
  scene.add(frame.object);
  const renderer = new THREE.WebGLRenderer({ antialias: true });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  document.body.append(renderer.domElement);
  const camera = new THREE.PerspectiveCamera(45, 1, 0.01, 100);
  const controls = new OrbitControls(camera, renderer.domElement);
  // 模型和书本可以超出单位格子，按实际几何范围取景。
  const center: THREE.Vector3 = frame.bounds.getCenter(new THREE.Vector3());
  controls.target.copy(center);
  camera.position.copy(center).add(new THREE.Vector3(2.2, 1.6, 2.7));
  controls.update();

  function resize(): void {
    renderer.setSize(window.innerWidth, window.innerHeight);
    camera.aspect = window.innerWidth / window.innerHeight;
    camera.updateProjectionMatrix();
  }
  window.addEventListener("resize", resize);
  resize();
  // 4. 回调给出毫秒，updateTime 接收秒；水、火焰和书本共用这个时钟。
  renderer.setAnimationLoop((time: number) => {
    frame.updateTime(time / 1000);
    renderer.render(scene, camera);
  });
}

main().catch((error: unknown) => {
  console.error(error);
  document.body.textContent = String(error);
});
