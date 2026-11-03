import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { createMeshObject, decodeMesh, loadAtlas, type MeshData, type MeshObject } from "@neomega/viewer/render";

async function main(): Promise<void> {
  // 1. 网格中的 UV 指向这张配套图集；贴图与 Go 使用的几何表来自同一份资源。
  const atlas: THREE.Texture = await loadAtlas("/textures/atlas.png");
  // 2. 请求 Go 生成的二进制结果，解码后创建可以加入 Three.js 场景的对象。
  const response: Response = await fetch("/api/subchunk?subchunk=default");
  if (!response.ok) throw new Error(await response.text());
  const binary: ArrayBuffer = await response.arrayBuffer();
  const meshData: MeshData = decodeMesh(binary); // 对应 Go 的 Mesh.Marshal（NVMS）。
  const mesh: MeshObject = createMeshObject(meshData, atlas);

  // 3. 以下相机、背景和鼠标控制只决定本示例的取景。
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0xeeeeee);
  scene.add(mesh.object);

  const renderer = new THREE.WebGLRenderer({ antialias: true });
  document.body.append(renderer.domElement);
  const camera = new THREE.PerspectiveCamera(45, 1, 0.1, 100);
  camera.position.set(25, 20, 29);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.target.set(8, 4, 8);
  controls.update();

  // 4. 本例只有静态材质，相机改变或窗口缩放时重绘即可。
  function render(): void {
    renderer.render(scene, camera);
  }
  function resize(): void {
    renderer.setSize(window.innerWidth, window.innerHeight);
    camera.aspect = window.innerWidth / window.innerHeight;
    camera.updateProjectionMatrix();
    render();
  }
  controls.addEventListener("change", render);
  window.addEventListener("resize", resize);
  resize();
}

main().catch((error: unknown) => {
  console.error(error);
  document.body.textContent = String(error);
});
