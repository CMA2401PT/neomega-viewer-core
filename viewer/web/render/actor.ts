import * as THREE from "three";
import { createBatchObject, type MeshOptions } from "./mesh.ts";
import type { Batch, BlockActor } from "./types.ts";
import type { RenderObject } from "./object.ts";

/** 绘制已经烘焙的外观；只认识网格和图片，不读取方块名或 NBT。私有贴图随对象释放。 */
export function createActorObject(
  actor: BlockActor,
  atlas: THREE.Texture,
  options: MeshOptions = {},
): RenderObject {
  const textures = new Map<string, THREE.DataTexture>();
  for (const image of actor.textures ?? []) {
    // 拷贝小图片，避免卸载其他通道后仍由它持有整份 NVFR 响应。
    const texture = new THREE.DataTexture(image.pixels.slice(), image.width, image.height);
    texture.colorSpace = THREE.SRGBColorSpace;
    texture.flipY = false;
    texture.magFilter = texture.minFilter = THREE.NearestFilter;
    texture.generateMipmaps = false;
    texture.needsUpdate = true;
    textures.set(image.name, texture);
  }
  // 只有采样同一张图片的批次才能合并；同名私有贴图的作用域始终是当前 actor。
  const groups = new Map<THREE.Texture, Batch[]>();
  for (const batch of actor.mesh.batches) {
    const texture = textures.get(batch.surface.texture) ?? atlas;
    const group = groups.get(texture) ?? [];
    group.push(batch);
    groups.set(texture, group);
  }
  const parts: RenderObject[] = [];
  const object = new THREE.Group();
  const bounds = new THREE.Box3();
  for (const [texture, batches] of groups) {
    const part = createBatchObject(batches, texture, {
      ...options,
      // 世界共享材质只绑定公共图集；私有图片的材质归当前对象所有。
      materials: texture === atlas ? options.materials : undefined,
    });
    parts.push(part);
    object.add(part.object);
    bounds.union(part.bounds);
  }
  return {
    object,
    bounds,
    animated: parts.some(part => part.animated),
    updateTime(seconds: number) { for (const part of parts) part.updateTime(seconds); },
    dispose() {
      for (const part of parts) part.dispose();
      for (const texture of textures.values()) texture.dispose();
    },
  };
}
