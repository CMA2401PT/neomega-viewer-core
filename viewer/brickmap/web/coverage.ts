import type { Vec3 } from "./volume.ts";

export interface CoveredSection { x: number; y: number; z: number }

export class BrickmapCoverage {
  readonly texture: WebGLTexture;
  readonly size: Vec3;
  readonly min: Vec3 = [0, 0, 0];
  readonly max: Vec3 = [0, 0, 0];
  private readonly cells: Uint8Array;
  private readonly gl: WebGL2RenderingContext;

  constructor(gl: WebGL2RenderingContext, size: Vec3) {
    // 以 16³ section 为单位让出精细网格，空 section 同样是已知覆盖范围。
    this.gl = gl;
    this.size = size.map(n => n / 16) as Vec3;
    this.cells = new Uint8Array(this.size[0] * this.size[1] * this.size[2]);
    this.texture = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_3D, this.texture);
    gl.texStorage3D(gl.TEXTURE_3D, 1, gl.R8UI, ...this.size);
    gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
  }

  update(origin: Vec3, sections: Iterable<CoveredSection>): void {
    // 网格挂载、卸载或窗口移动后重建；边界盒让远处的均匀节点仍能整段跳过。
    this.cells.fill(0);
    this.min.fill(Infinity);
    this.max.fill(0);
    const [sx, sy, sz] = this.size;
    for (const section of sections) {
      const x = section.x - origin[0] / 16;
      const y = section.y - origin[1] / 16;
      const z = section.z - origin[2] / 16;
      if (x < 0 || x >= sx || y < 0 || y >= sy || z < 0 || z >= sz) continue;
      this.cells[x + sx * (y + sy * z)] = 1;
      [x, y, z].forEach((n, axis) => {
        this.min[axis] = Math.min(this.min[axis], n * 16);
        this.max[axis] = Math.max(this.max[axis], (n + 1) * 16);
      });
    }
    if (this.min[0] === Infinity) this.min.fill(0);
    const gl = this.gl;
    gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
    gl.bindTexture(gl.TEXTURE_3D, this.texture);
    gl.texSubImage3D(gl.TEXTURE_3D, 0, 0, 0, 0, sx, sy, sz, gl.RED_INTEGER, gl.UNSIGNED_BYTE, this.cells);
  }

  dispose(): void {
    this.gl.deleteTexture(this.texture);
  }
}
