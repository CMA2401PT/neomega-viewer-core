export type Vec3 = [number, number, number];
export interface Volume {
  origin: Vec3;
  size: Vec3;
  levels: { size: Vec3; cells: Uint32Array }[];
  palette: Uint8Array;
  groups: Uint32Array;
  bricks: Uint16Array;
}

/** 解析 Go Volume.Write 的连续数据；所有数组借用同一个响应缓冲。 */
export function decodeVolume(buffer: ArrayBuffer): Volume {
  const headerBytes = new DataView(buffer).getUint32(0, true);
  const header = JSON.parse(new TextDecoder().decode(new Uint8Array(buffer, 4, headerBytes))) as {
    format: string; origin: Vec3; size: Vec3; levels: Vec3[]; brickBytes: number; materialCount: number;
  };
  if (header.format !== "brickmap-2") throw new Error(`不支持的体素格式：${header.format}`);
  let offset = 4 + headerBytes;
  const palette = new Uint8Array(buffer, offset, header.materialCount * 24);
  offset += header.materialCount * 24;
  const groups = new Uint32Array(buffer, offset, header.materialCount);
  offset += header.materialCount * 4;
  const levels = header.levels.map(size => {
    const cells = new Uint32Array(buffer, offset, size[0] * size[1] * size[2]);
    offset += cells.byteLength;
    return { size, cells };
  });
  return { origin: header.origin, size: header.size, levels, palette, groups, bricks: new Uint16Array(buffer, offset, header.brickBytes / 2) };
}

/** 返回世界坐标处的颜色编号，供独立的逐格参考算法使用。 */
export function voxelAt(volume: Volume, position: Vec3): number {
  const p = position.map((c, i) => c - volume.origin[i]);
  if (p.some((c, i) => c < 0 || c >= volume.size[i])) return 0;
  const { size, cells } = volume.levels[0];
  const brick = cells[(p[0] >> 3) + size[0] * ((p[1] >> 3) + size[1] * (p[2] >> 3))];
  return brick < 65536 ? brick : volume.bricks[(brick - 65536) * 512 + (p[0] & 7) + 8 * ((p[1] & 7) + 8 * (p[2] & 7))];
}

export interface Camera {
  target: Vec3;
  yaw: number;
  pitch: number;
  /** 屏幕高度对应的世界单位数，所有距离的方块具有相同投影尺寸。 */
  span: number;
}

/** 正交相机的所有射线共用方向，仅起点随像素改变；角度使用度。 */
export function cameraBasis(camera: Camera): { right: Vec3; up: Vec3; direction: Vec3 } {
  const yaw = camera.yaw * Math.PI / 180, pitch = camera.pitch * Math.PI / 180;
  return {
    right: [Math.cos(yaw), 0, -Math.sin(yaw)],
    up: [-Math.sin(yaw) * Math.sin(pitch), Math.cos(pitch), -Math.cos(yaw) * Math.sin(pitch)],
    direction: [-Math.sin(yaw) * Math.cos(pitch), -Math.sin(pitch), -Math.cos(yaw) * Math.cos(pitch)],
  };
}
