import type { Vec3, Volume } from "./volume.ts";

export const MIXED = 0xffffffff;
export const UNKNOWN = 0xfffffffe;
const PAGE_SIDE = 2048;
const PAGE_CELLS = PAGE_SIDE * PAGE_SIDE;
const BRICK_CELLS = 512;

type Level = Volume["levels"][number];

export interface GPURegion {
  // 目录保留区域本地编号；窗口换位置时只重新映射，不改写原始目录。
  readonly origin: Vec3;
  readonly size: Vec3;
  readonly levels: readonly Level[];
  readonly materials: Uint16Array;
  readonly slots: Uint32Array;
}

export class BrickmapPool {
  readonly gl: WebGL2RenderingContext;
  readonly bricks: WebGLTexture;
  readonly palette: WebGLTexture;
  readonly groups: WebGLTexture;
  readonly textureBytes: number;
  readonly capacity: number;
  private readonly materialIDs = new Map<string, number>();
  private readonly free: number[] = [];
  private nextSlot = 0;
  private nextMaterial = 1;

  constructor(gl: WebGL2RenderingContext, brickBytes: number = 512 * 1024 ** 2) {
    // 多个窗口借用同一砖池与调色板；池的寿命由持有这些窗口的宿主管理。
    this.gl = gl;
    const pages = Math.max(1, Math.ceil(brickBytes / (PAGE_CELLS * 2)));
    if (pages > gl.getParameter(gl.MAX_ARRAY_TEXTURE_LAYERS)) throw new Error("砖池预算超过设备纹理层数");
    this.capacity = pages * PAGE_CELLS / BRICK_CELLS;
    this.textureBytes = pages * PAGE_CELLS * 2 + 65536 * 28;
    this.bricks = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, this.bricks);
    gl.texStorage3D(gl.TEXTURE_2D_ARRAY, 1, gl.R16UI, PAGE_SIDE, PAGE_SIDE, pages);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    this.palette = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D, this.palette);
    gl.texStorage2D(gl.TEXTURE_2D, 1, gl.RGBA8, 1536, 256);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    this.groups = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D, this.groups);
    gl.texStorage2D(gl.TEXTURE_2D, 1, gl.R32UI, 256, 256);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
  }

  upload(volume: Volume): GPURegion | null {
    // 在帧内调用；容量不足时不分配任何槽位，调用方保留“未装入”状态。
    const count = volume.bricks.length / BRICK_CELLS;
    if (count > this.capacity - this.nextSlot + this.free.length) return null;
    const keys = Array.from(volume.groups, (group, id) =>
      `${group}/${volume.palette.subarray(id * 24, (id + 1) * 24).join(",")}`);
    const additions = new Set(keys.slice(1).filter(key => !this.materialIDs.has(key)));
    if (this.nextMaterial + additions.size > 65536) return null;
    const gl = this.gl;
    gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, false);
    gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false);
    gl.pixelStorei(gl.UNPACK_ROW_LENGTH, 0);
    gl.pixelStorei(gl.UNPACK_IMAGE_HEIGHT, 0);
    gl.pixelStorei(gl.UNPACK_SKIP_PIXELS, 0);
    gl.pixelStorei(gl.UNPACK_SKIP_ROWS, 0);
    gl.pixelStorei(gl.UNPACK_SKIP_IMAGES, 0);
    const materials = new Uint16Array(keys.length);
    for (let local = 1; local < keys.length; local++) {
      let id = this.materialIDs.get(keys[local]);
      if (id === undefined) {
        id = this.nextMaterial++;
        this.materialIDs.set(keys[local], id);
        gl.bindTexture(gl.TEXTURE_2D, this.palette);
        gl.texSubImage2D(gl.TEXTURE_2D, 0, (id & 255) * 6, id >> 8, 6, 1,
          gl.RGBA, gl.UNSIGNED_BYTE, volume.palette.subarray(local * 24, (local + 1) * 24));
        gl.bindTexture(gl.TEXTURE_2D, this.groups);
        gl.texSubImage2D(gl.TEXTURE_2D, 0, id & 255, id >> 8, 1, 1,
          gl.RED_INTEGER, gl.UNSIGNED_INT, volume.groups.subarray(local, local + 1));
      }
      materials[local] = id;
    }
    const slots = new Uint32Array(count);
    for (let i = 0; i < count; i++) slots[i] = this.free.pop() ?? this.nextSlot++;
    const mapped = Uint16Array.from(volume.bricks, id => materials[id]);
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, this.bricks);
    // 相邻槽位合成连续上传，避免为每个 8³ 小块发起一次 GL 调用。
    for (let start = 0; start < count;) {
      let end = start + 1;
      while (end < count && slots[end] === slots[end - 1] + 1) end++;
      let offset = start * BRICK_CELLS;
      let address = slots[start] * BRICK_CELLS;
      const stop = end * BRICK_CELLS;
      while (offset < stop) {
        const page = Math.floor(address / PAGE_CELLS);
        const x = address % PAGE_SIDE;
        const y = Math.floor(address / PAGE_SIDE) % PAGE_SIDE;
        const width = x !== 0 || stop - offset < PAGE_SIDE ? Math.min(PAGE_SIDE - x, stop - offset) : PAGE_SIDE;
        const height = width === PAGE_SIDE ? Math.min(PAGE_SIDE - y, Math.floor((stop - offset) / PAGE_SIDE)) : 1;
        const cells = width * height;
        gl.texSubImage3D(gl.TEXTURE_2D_ARRAY, 0, x, y, page, width, height, 1,
          gl.RED_INTEGER, gl.UNSIGNED_SHORT, mapped.subarray(offset, offset + cells));
        offset += cells;
        address += cells;
      }
      start = end;
    }
    return {
      origin: [...volume.origin],
      size: [...volume.size],
      levels: volume.levels.map(level => ({ size: [...level.size], cells: level.cells.slice() })),
      materials,
      slots,
    };
  }

  release(region: GPURegion): void {
    // 先从所有窗口摘除目录引用，再归还槽位；同一上下文的命令顺序保证复用安全。
    for (const slot of region.slots) this.free.push(slot);
  }

  dispose(): void {
    // 窗口和绘制器先结束使用，共享纹理只由池释放一次。
    this.gl.deleteTexture(this.bricks);
    this.gl.deleteTexture(this.palette);
    this.gl.deleteTexture(this.groups);
  }
}

export function directoryLevels(size: Vec3): Level[] {
  // 目录补齐到二次幂，保证每级都能完整合并；窗口的实际射线范围仍由 size 限定。
  // 补齐与未到达区域使用 UNKNOWN，空气编号 0 只表示已知为空。
  const levels: Level[] = [];
  let dimensions = size.map(n => 2 ** Math.ceil(Math.log2(n / 8))) as Vec3;
  for (;;) {
    levels.push({ size: dimensions, cells: new Uint32Array(dimensions[0] * dimensions[1] * dimensions[2]).fill(UNKNOWN) });
    if (dimensions.every(n => n === 1)) return levels;
    dimensions = dimensions.map(n => Math.max(1, n / 2)) as Vec3;
  }
}

export function mergeParents(levels: Level[], from: number): void {
  // 未知可以整段跳出；砖指针只在最细层有效，父层一律标记为 Mixed。
  for (let level = from; level < levels.length; level++) {
    const previous = levels[level - 1];
    const current = levels[level];
    const [px, py, pz] = previous.size;
    const [sx, sy, sz] = current.size;
    for (let z = 0; z < sz; z++) for (let y = 0; y < sy; y++) for (let x = 0; x < sx; x++) {
      let value = previous.cells[x * 2 + px * (y * 2 + py * z * 2)];
      for (let dz = 0; dz < Math.min(2, pz); dz++) for (let dy = 0; dy < Math.min(2, py); dy++) for (let dx = 0; dx < Math.min(2, px); dx++) {
        const child = previous.cells[x * 2 + dx + px * (y * 2 + dy + py * (z * 2 + dz))];
        if (value !== child || (child >= 65536 && child !== UNKNOWN)) value = MIXED;
      }
      current.cells[x + sx * (y + sy * z)] = value;
    }
  }
}

export class BrickmapWindow {
  readonly gl: WebGL2RenderingContext;
  readonly texture: WebGLTexture;
  readonly size: Vec3;
  readonly levels: Level[];
  origin: Vec3;

  constructor(gl: WebGL2RenderingContext, origin: Vec3, size: Vec3) {
    // 一个窗口拥有一份目录，砖与材质来自独立的共享池。
    this.gl = gl;
    this.origin = [...origin];
    this.size = [...size];
    this.levels = directoryLevels(size);
    const maximum = gl.getParameter(gl.MAX_3D_TEXTURE_SIZE) as number;
    if (this.levels[0].size.some(n => n > maximum)) throw new Error(`目录超过设备的三维纹理边长 ${maximum}`);
    this.texture = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_3D, this.texture);
    gl.texStorage3D(gl.TEXTURE_3D, this.levels.length, gl.R32UI, ...this.levels[0].size);
    gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_MIN_FILTER, gl.NEAREST_MIPMAP_NEAREST);
    gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    this.uploadLevels(0);
  }

  rebuild(origin: Vec3, regions: readonly GPURegion[]): void {
    // 窗口跳动时重建目录；重叠区域沿用原来的砖池槽位。
    this.origin = [...origin];
    for (const level of this.levels) level.cells.fill(UNKNOWN);
    for (const region of regions) this.writeRegion(region, false, false);
    mergeParents(this.levels, 5);
    this.uploadLevels(0);
  }

  install(region: GPURegion): void {
    // 只有砖池上传完成的区域才可发布；高层目录很小，每次完整合并。
    this.writeRegion(region, false, true);
    mergeParents(this.levels, 5);
    this.uploadLevels(5);
  }

  remove(region: GPURegion): void {
    // 摘除叶层和父层后，宿主才能把区域占用的槽位交还共享池。
    this.writeRegion(region, true, true);
    mergeParents(this.levels, 5);
    this.uploadLevels(5);
  }

  load(region: GPURegion): void {
    // 静态体积覆盖整份目录，供离线预览使用同一套共享资源与绘制接口。
    this.origin = [...region.origin];
    for (let i = 0; i < this.levels.length; i++) {
      this.levels[i].cells.set(this.remap(region, i));
    }
    this.uploadLevels(0);
  }

  private remap(region: GPURegion, level: number): Uint32Array {
    // 均匀节点与混合砖都引用区域本地编号，映射时必须同时处理。
    return Uint32Array.from(region.levels[level].cells, value =>
      value < 65536 ? region.materials[value] : value === MIXED ? MIXED : 65536 + region.slots[value - 65536]);
  }

  private writeRegion(region: GPURegion, erase: boolean, upload: boolean): void {
    // 128 格区域在 0…4 级完全对齐，第 5 级开始合并相邻区域。
    const gl = this.gl;
    gl.bindTexture(gl.TEXTURE_3D, this.texture);
    for (let i = 0; i <= 4; i++) {
      const source = region.levels[i];
      const target = this.levels[i];
      const offset = region.origin.map((n, axis) => (n - this.origin[axis]) / (8 * 2 ** i)) as Vec3;
      const cells = erase ? new Uint32Array(source.cells.length).fill(UNKNOWN) : this.remap(region, i);
      for (let z = 0; z < source.size[2]; z++) for (let y = 0; y < source.size[1]; y++) {
        const start = source.size[0] * (y + source.size[1] * z);
        const destination = offset[0] + target.size[0] * (offset[1] + y + target.size[1] * (offset[2] + z));
        target.cells.set(cells.subarray(start, start + source.size[0]), destination);
      }
      if (upload) gl.texSubImage3D(gl.TEXTURE_3D, i, ...offset, ...source.size, gl.RED_INTEGER, gl.UNSIGNED_INT, cells);
    }
  }

  private uploadLevels(from: number): void {
    // 每级数组按 X 最快排列，与三维纹理的连续行一致。
    const gl = this.gl;
    gl.bindTexture(gl.TEXTURE_3D, this.texture);
    for (let i = from; i < this.levels.length; i++) {
      const level = this.levels[i];
      gl.texSubImage3D(gl.TEXTURE_3D, i, 0, 0, 0, ...level.size, gl.RED_INTEGER, gl.UNSIGNED_INT, level.cells);
    }
  }

  dispose(): void {
    // 只释放本窗口目录，共享砖池仍可供其他窗口使用。
    this.gl.deleteTexture(this.texture);
  }
}
