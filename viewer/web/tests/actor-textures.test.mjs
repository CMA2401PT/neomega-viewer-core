import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { Texture, Scene, PerspectiveCamera } from "three";
import { decodeFrame, createFrameObject, SectionScene } from "@neomega/viewer/render";

function fixture() {
  const bytes = readFileSync(new URL("./fixtures/banners.nvframe", import.meta.url));
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
}

function privateTextures(object) {
  const textures = new Set();
  object.traverse(draw => {
    const texture = draw.material?.uniforms?.atlasMap?.value;
    if (texture?.isDataTexture) textures.add(texture);
  });
  return textures;
}

test("Go v2 快照保留两面旗帜的同名私有贴图，像素和网格借用响应缓冲", () => {
  const buffer = fixture();
  assert.equal(new DataView(buffer).getUint32(4, true), 2);
  const frame = decodeFrame(buffer);
  assert.equal(frame.mesh.quads, 0);
  assert.equal(frame.actors.length, 2);
  const [first, second] = frame.actors.map(actor => actor.textures[0]);
  assert.equal(first.name, second.name);
  assert.equal(first.width, 64);
  assert.equal(first.height, 64);
  assert.equal(first.pixels.buffer, buffer);
  assert.notDeepEqual(first.pixels, second.pixels);
  for (const actor of frame.actors) {
    assert(actor.mesh.bounds.max[1] > actor.pos[1] + 1);
    for (const batch of actor.mesh.batches) assert.equal(batch.positions.buffer, buffer);
  }
  assert.throws(() => decodeFrame(buffer.slice(0, -4)));
  const broken = fixture();
  const view = new DataView(broken);
  const actorOffset = view.getUint32(40, true);
  const meshLength = view.getUint32(actorOffset + 8, true);
  const firstTexture = actorOffset + 4 + 8 + meshLength + 4;
  view.setUint32(firstTexture + 4, 0xffffffff, true);
  assert.throws(() => decodeFrame(broken), /贴图不完整/);
});

test("独立快照按 actor 绑定图片，释放自有贴图及材质，不释放公共图集", () => {
  const buffer = fixture();
  const data = decodeFrame(buffer);
  const atlas = new Texture({ width: 2048, height: 1024 });
  let atlasDisposed = 0, textureDisposed = 0, materialDisposed = 0;
  atlas.addEventListener("dispose", () => atlasDisposed++);
  const frame = createFrameObject(data, atlas, new Map(), { batchOpaqueSurfaces: true });
  const textures = privateTextures(frame.object);
  assert.equal(textures.size, 4);
  for (const texture of textures) {
    assert.notEqual(texture.image.data.buffer, buffer);
    texture.addEventListener("dispose", () => textureDisposed++);
  }
  const materials = new Set();
  frame.object.traverse(draw => { if (draw.isMesh) materials.add(draw.material); });
  for (const material of materials) material.addEventListener("dispose", () => materialDisposed++);
  frame.dispose();
  assert.equal(textureDisposed, 4);
  assert.equal(materialDisposed, materials.size);
  assert.equal(atlasDisposed, 0);
});

test("世界中的私有贴图随区域卸载，重载与另一地区域不会串图", () => {
  const data = decodeFrame(fixture());
  const atlas = new Texture({ width: 2048, height: 1024 });
  const scene = new Scene();
  const world = new SectionScene(scene, atlas, new Map(), undefined, 16);
  const first = world.prepare({ x: -1, y: 0, z: 2 }, data);
  const second = world.prepare({ x: 1, y: 0, z: 2 }, data);
  world.add(first); world.add(second);
  const a = privateTextures(first.frame.object), b = privateTextures(second.frame.object);
  assert.equal(a.size, 4); assert.equal(b.size, 4);
  assert([...a].every(texture => !b.has(texture)));
  let aDisposed = 0, bDisposed = 0;
  for (const texture of a) texture.addEventListener("dispose", () => aDisposed++);
  for (const texture of b) texture.addEventListener("dispose", () => bDisposed++);
  world.update(new PerspectiveCamera(), 2);
  assert(first.bounds.max.y > 2);
  first.dispose();
  assert.equal(aDisposed, 4); assert.equal(bDisposed, 0);
  const again = world.prepare({ x: -1, y: 0, z: 2 }, data);
  world.add(again);
  assert([...privateTextures(again.frame.object)].every(texture => !a.has(texture)));
  world.dispose();
  assert.equal(bDisposed, 4);
});
