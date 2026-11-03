import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import {
  createEffectRegistry,
  createFrameObject,
  decodeFrame,
  loadAtlas,
} from "@neomega/viewer/render";

async function main(): Promise<void> {
  const name = new URLSearchParams(location.search).get("scene") ?? "waterlogged_leaves";
  const atlas = await loadAtlas("/textures/atlas.png");
  const book = await loadAtlas("/textures/entity/enchanting_table_book.png");
  const response = await fetch(`/api/block?scene=${encodeURIComponent(name)}`);
  if (!response.ok) throw new Error(await response.text());
  // 四种场景共用同一条解码和绘制链路，网页无需区分方块类型。
  const data = decodeFrame(await response.arrayBuffer());
  const frame = createFrameObject(data, atlas, createEffectRegistry(book));

  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0xeeeeee);
  scene.add(frame.object);
  const renderer = new THREE.WebGLRenderer({ antialias: true });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  document.body.append(renderer.domElement);
  const camera = new THREE.PerspectiveCamera(45, 1, 0.01, 100);
  const controls = new OrbitControls(camera, renderer.domElement);
  const center = frame.bounds.getCenter(new THREE.Vector3());
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
  // 火焰和水的纹理动画、书本的几何动画，都使用这个秒数。
  renderer.setAnimationLoop((time: number) => {
    frame.updateTime(time / 1000);
    renderer.render(scene, camera);
  });
}

main().catch((error: unknown) => {
  console.error(error);
  document.body.textContent = String(error);
});
