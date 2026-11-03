import * as THREE from "three";
import { createActorObject } from "./actor.ts";
import { createBatchObject, type MeshOptions } from "./mesh.ts";
import { createEffectsObject, type EffectRegistry } from "./effects/index.ts";
import type { RenderObject } from "./object.ts";
import type { FrameData } from "./types.ts";

export type FrameObject = RenderObject;

/** 三段作为同一场景中的普通对象，共享深度缓冲和透明排序。 */
export function createFrameObject(
  data: FrameData,
  atlas: THREE.Texture,
  registry: EffectRegistry = new Map(),
  options: MeshOptions = {},
): FrameObject {
  const object = new THREE.Group();
  const bounds = new THREE.Box3();
  const meshes = [
    createBatchObject(data.mesh.batches, atlas, options),
    ...data.actors.map(actor => createActorObject(actor, atlas, options)),
  ];
  for (const mesh of meshes) bounds.union(mesh.bounds);
  const effects = createEffectsObject(data.effects, registry);
  bounds.union(effects.bounds);
  for (const mesh of meshes) object.add(mesh.object);
  for (const effect of [...effects.object.children]) object.add(effect);
  if (bounds.isEmpty()) bounds.set(new THREE.Vector3(), new THREE.Vector3(1, 1, 1));
  return {
    object,
    bounds,
    animated: effects.animated || meshes.some((mesh) => mesh.animated),
    updateTime(seconds: number) {
      meshes.forEach((mesh) => mesh.updateTime(seconds));
      effects.updateTime(seconds);
    },
    dispose() {
      meshes.forEach((mesh) => mesh.dispose());
      effects.dispose();
    },
  };
}
