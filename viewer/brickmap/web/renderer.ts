import { cameraBasis, type Camera, type Vec3 } from "./volume.ts";
import { BrickmapPool, BrickmapWindow } from "./gpu.ts";
import type { BrickmapCoverage } from "./coverage.ts";
import { makeFragmentShader, vertexShader } from "./shaders.ts";

interface TimerExtension { TIME_ELAPSED_EXT: number; GPU_DISJOINT_EXT: number }
export interface Ray { origin: Vec3; direction: Vec3 }
export type PickResult =
  | { kind: "hit"; position: Vec3; face: number }
  | { kind: "missing"; position: Vec3 }
  | { kind: "miss" };
export interface PerspectiveCamera {
  position: Vec3;
  right: Vec3;
  up: Vec3;
  direction: Vec3;
  verticalFov: number;
  near: number;
  far: number;
}
export interface PerspectiveDrawOptions extends DrawOptions {
  coverage?: BrickmapCoverage;
  fog: [number, number];
}
export interface DrawOptions {
  viewport?: [number, number, number, number];
  framebuffer?: WebGLFramebuffer | null;
  clipY?: number;
  openBoundary?: boolean;
  background?: Vec3;
  hierarchy?: boolean;
  debug?: number;
  timing?: boolean;
}
interface Program {
  handle: WebGLProgram;
  uniforms: Map<string, WebGLUniformLocation | null>;
}

export function cameraRay(camera: Camera, width: number, height: number, x: number, y: number): Ray {
  // 返回经过屏幕点的正交射线；起点位于目标平面，调用方可沿射线移到体积前方。
  const { right, up, direction } = cameraBasis(camera);
  const u = (x - width / 2) * camera.span / height;
  const v = (height / 2 - y) * camera.span / height;
  return { origin: camera.target.map((n, i) => n + right[i] * u + up[i] * v) as Vec3, direction };
}

export class BrickmapRenderer {
  readonly gl: WebGL2RenderingContext;
  readonly gpuTimes: number[] = [];
  readonly pool: BrickmapPool;
  readonly window: BrickmapWindow;
  private readonly color: Program;
  private readonly picking: Program;
  private perspective?: Program;
  private readonly vao: WebGLVertexArrayObject;
  private readonly pickTarget: WebGLFramebuffer;
  private readonly pickTexture: WebGLTexture;
  private readonly timer: TimerExtension | null;
  private readonly queries: WebGLQuery[] = [];

  constructor(gl: WebGL2RenderingContext, pool: BrickmapPool, window: BrickmapWindow) {
    // 绘制器借用窗口与共享资源；宿主决定何时上传、绘制和交还上下文。
    this.gl = gl;
    this.pool = pool;
    this.window = window;
    this.color = this.compile(false);
    this.picking = this.compile(true);
    this.vao = gl.createVertexArray()!;
    this.pickTarget = gl.createFramebuffer()!;
    this.pickTexture = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D, this.pickTexture);
    gl.texStorage2D(gl.TEXTURE_2D, 1, gl.RGBA32I, 1, 1);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.bindFramebuffer(gl.FRAMEBUFFER, this.pickTarget);
    gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, this.pickTexture, 0);
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    this.timer = gl.getExtension("EXT_disjoint_timer_query_webgl2") as TimerExtension | null;
  }

  get textureBytes(): number {
    return this.pool.textureBytes + this.window.levels.reduce((sum, level) => sum + level.cells.byteLength, 0);
  }

  private compile(picking: boolean, perspective = false): Program {
    // 两种输出共享遍历源码，拾取不会维护另一套边界或透明介质规则。
    const gl = this.gl;
    const handle = gl.createProgram()!;
    for (const [type, source] of [[gl.VERTEX_SHADER, vertexShader], [gl.FRAGMENT_SHADER, makeFragmentShader(picking, perspective)]] as const) {
      const shader = gl.createShader(type)!;
      gl.shaderSource(shader, source);
      gl.compileShader(shader);
      if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(shader)!);
      gl.attachShader(handle, shader);
      gl.deleteShader(shader);
    }
    gl.linkProgram(handle);
    if (!gl.getProgramParameter(handle, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(handle)!);
    const uniforms = new Map<string, WebGLUniformLocation | null>();
    for (const name of ["uDirectory", "uBricks", "uPalette", "uGroups", "uSize", "uOrigin", "uRight", "uUp", "uDirection", "uViewport", "uSpan", "uFar", "uTop", "uDebug", "uClipY", "uOpenBoundary", "uBackground", "uNear", "uCoverage", "uUseCoverage", "uCoverageMin", "uCoverageMax", "uFog"]) {
      uniforms.set(name, gl.getUniformLocation(handle, name));
    }
    return { handle, uniforms };
  }

  private bind(program: Program, options: DrawOptions): void {
    // three 或其他绘制器会改写这些状态，每次提交都完整建立本次所需状态。
    const gl = this.gl;
    const viewport = options.viewport ?? [0, 0, gl.drawingBufferWidth, gl.drawingBufferHeight];
    gl.bindFramebuffer(gl.FRAMEBUFFER, options.framebuffer ?? null);
    gl.viewport(...viewport);
    gl.scissor(...viewport);
    gl.enable(gl.SCISSOR_TEST);
    gl.disable(gl.DEPTH_TEST);
    gl.depthMask(false);
    gl.disable(gl.BLEND);
    gl.disable(gl.CULL_FACE);
    gl.disable(gl.STENCIL_TEST);
    gl.disable(gl.POLYGON_OFFSET_FILL);
    gl.disable(gl.SAMPLE_ALPHA_TO_COVERAGE);
    gl.disable(gl.RASTERIZER_DISCARD);
    gl.colorMask(true, true, true, true);
    gl.useProgram(program.handle);
    gl.bindVertexArray(this.vao);
    const textures = [
      [gl.TEXTURE_3D, this.window.texture, "uDirectory"],
      [gl.TEXTURE_2D_ARRAY, this.pool.bricks, "uBricks"],
      [gl.TEXTURE_2D, this.pool.palette, "uPalette"],
      [gl.TEXTURE_2D, this.pool.groups, "uGroups"],
    ] as const;
    textures.forEach(([target, texture, name], i) => {
      gl.activeTexture(gl.TEXTURE0 + i);
      gl.bindTexture(target, texture);
      gl.bindSampler(i, null);
      gl.uniform1i(program.uniforms.get(name)!, i);
    });
    const u = program.uniforms;
    gl.uniform4fv(u.get("uViewport")!, viewport);
    gl.uniform3fv(u.get("uSize")!, this.window.size);
    gl.uniform1f(u.get("uClipY")!, (options.clipY ?? this.window.origin[1] + this.window.size[1]) - this.window.origin[1]);
    gl.uniform1i(u.get("uOpenBoundary")!, options.openBoundary ? 1 : 0);
    gl.uniform1i(u.get("uTop")!, options.hierarchy === false ? 0 : this.window.levels.length - 1);
    gl.uniform1i(u.get("uDebug")!, options.debug ?? 0);
    gl.uniform3fv(u.get("uBackground")!, options.background ?? [0.055, 0.075, 0.095]);
  }

  draw(camera: Camera, options: DrawOptions = {}): void {
    // 小地图在主场景之后每帧绘制；全屏三角形自行输出背景，无需清除其他视口。
    const gl = this.gl;
    this.bind(this.color, options);
    const { right, up, direction } = cameraBasis(camera);
    const { origin: base, size } = this.window;
    const depth = size.reduce((sum, n, i) => sum + Math.abs(direction[i]) * n, 0);
    const distance = camera.target.reduce((sum, n, i) => sum + (n - base[i] - size[i] / 2) * direction[i], 0) + depth / 2 + 1;
    const u = this.color.uniforms;
    gl.uniform3fv(u.get("uOrigin")!, camera.target.map((n, i) => n - base[i] - direction[i] * distance));
    gl.uniform3fv(u.get("uRight")!, right);
    gl.uniform3fv(u.get("uUp")!, up);
    gl.uniform3fv(u.get("uDirection")!, direction);
    gl.uniform1f(u.get("uSpan")!, camera.span);
    gl.uniform1f(u.get("uFar")!, depth + 2);
    const timer = this.timer;
    if (timer && gl.getParameter(timer.GPU_DISJOINT_EXT)) this.resetTiming();
    while (this.queries.length && gl.getQueryParameter(this.queries[0], gl.QUERY_RESULT_AVAILABLE)) {
      const query = this.queries.shift()!;
      this.gpuTimes.push((gl.getQueryParameter(query, gl.QUERY_RESULT) as number) / 1e6);
      if (this.gpuTimes.length > 240) this.gpuTimes.shift();
      gl.deleteQuery(query);
    }
    const query = timer && options.timing !== false && this.queries.length < 8 ? gl.createQuery() : null;
    if (query) gl.beginQuery(timer!.TIME_ELAPSED_EXT, query);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
    if (query) { gl.endQuery(timer!.TIME_ELAPSED_EXT); this.queries.push(query); }
  }

  drawPerspective(camera: PerspectiveCamera, options: PerspectiveDrawOptions): void {
    // 先写入与主相机一致的透视深度，宿主保留颜色与深度再绘制精细网格。
    const gl = this.gl;
    const program = this.perspective ??= this.compile(false, true);
    this.bind(program, options);
    gl.enable(gl.DEPTH_TEST);
    gl.depthFunc(gl.ALWAYS);
    gl.depthMask(true);
    const u = program.uniforms;
    gl.uniform3fv(u.get("uOrigin")!, camera.position.map((n, i) => n - this.window.origin[i]));
    gl.uniform3fv(u.get("uRight")!, camera.right);
    gl.uniform3fv(u.get("uUp")!, camera.up);
    gl.uniform3fv(u.get("uDirection")!, camera.direction);
    gl.uniform1f(u.get("uSpan")!, 2 * Math.tan(camera.verticalFov * Math.PI / 360));
    gl.uniform1f(u.get("uNear")!, camera.near);
    gl.uniform1f(u.get("uFar")!, camera.far);
    gl.uniform2fv(u.get("uFog")!, options.fog);
    const coverage = options.coverage;
    gl.activeTexture(gl.TEXTURE4);
    gl.bindTexture(gl.TEXTURE_3D, coverage?.texture ?? this.window.texture);
    gl.bindSampler(4, null);
    gl.uniform1i(u.get("uCoverage")!, 4);
    gl.uniform1i(u.get("uUseCoverage")!, coverage ? 1 : 0);
    gl.uniform3fv(u.get("uCoverageMin")!, coverage?.min ?? [0, 0, 0]);
    gl.uniform3fv(u.get("uCoverageMax")!, coverage?.max ?? [0, 0, 0]);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  }

  pick(ray: Ray, options: Pick<DrawOptions, "clipY" | "openBoundary"> = {}): PickResult {
    // 一次点击或头顶探测只读回一个整数像素，坐标不经过颜色编码或浮点深度反算。
    const gl = this.gl;
    this.bind(this.picking, { ...options, viewport: [0, 0, 1, 1], framebuffer: this.pickTarget });
    const u = this.picking.uniforms;
    gl.uniform3fv(u.get("uOrigin")!, ray.origin.map((n, i) => n - this.window.origin[i]));
    gl.uniform3fv(u.get("uDirection")!, ray.direction);
    gl.uniform3f(u.get("uRight")!, 0, 0, 0);
    gl.uniform3f(u.get("uUp")!, 0, 0, 0);
    gl.uniform1f(u.get("uSpan")!, 0);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
    const pixel = new Int32Array(4);
    gl.readPixels(0, 0, 1, 1, gl.RGBA_INTEGER, gl.INT, pixel);
    if (pixel[3] === -2) throw new Error("射线遍历超过上限");
    const position = [0, 1, 2].map(i => pixel[i] + this.window.origin[i]) as Vec3;
    return pixel[3] > 0 ? { kind: "hit", position, face: pixel[3] - 1 }
      : pixel[3] === -1 ? { kind: "missing", position } : { kind: "miss" };
  }

  resetTiming(): void {
    // 测量重启时清掉仍在途的查询，结果不混入下一个区间。
    for (const query of this.queries) this.gl.deleteQuery(query);
    this.queries.length = 0;
    this.gpuTimes.length = 0;
  }

  dispose(): void {
    // 借用的池和窗口由宿主释放，绘制器只拥有程序、拾取目标和查询。
    this.resetTiming();
    this.gl.deleteFramebuffer(this.pickTarget);
    this.gl.deleteTexture(this.pickTexture);
    this.gl.deleteVertexArray(this.vao);
    this.gl.deleteProgram(this.color.handle);
    this.gl.deleteProgram(this.picking.handle);
    if (this.perspective) this.gl.deleteProgram(this.perspective.handle);
  }
}
