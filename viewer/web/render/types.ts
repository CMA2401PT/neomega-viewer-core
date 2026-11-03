export type Vec3 = [number, number, number];
export type AlphaMode = "opaque" | "cutout" | "blend";
/** 区域以整数网格坐标定位；世界方块原点为此坐标乘以 SectionScene 约定的边长。 */
export interface SectionPos {
  x: number;
  y: number;
  z: number;
}
export interface Surface {
  texture: string;
  region: [number, number, number, number];
  mode: AlphaMode;
  doubleSided: boolean;
  unlit: boolean;
  opacity: number;
  frames: number;
  frameTime: number;
}
export type FloatAttribute = number[] | Float32Array;
export type IndexAttribute = number[] | Uint32Array;
export interface Batch {
  surface: Surface;
  positions: FloatAttribute;
  normals: FloatAttribute;
  shadeNormals: FloatAttribute;
  uvs: FloatAttribute;
  colors: FloatAttribute;
  indices: IndexAttribute;
}
export interface MeshData {
  batches: Batch[];
  bounds: { min: Vec3; max: Vec3 };
  quads: number;
}

export interface EffectMessage {
  pos: Vec3;
  effect: string;
}
/** actor 私有的非预乘 RGBA8、sRGB 图片；name 对应该 actor 的 Surface.texture。 */
export interface ActorTexture {
  name: string;
  width: number;
  height: number;
  pixels: Uint8Array;
}
export interface BlockActor {
  pos: Vec3;
  mesh: MeshData;
  textures?: ActorTexture[];
}
export interface FrameData {
  mesh: MeshData;
  effects: EffectMessage[];
  actors: BlockActor[];
}
