import { cameraBasis, type Camera, type Volume } from "./volume.ts";
import { fragmentShader, vertexShader } from "./shaders.ts";

interface TimerExtension { TIME_ELAPSED_EXT: number; GPU_DISJOINT_EXT: number }

/** 体素纹理一次上传，转动时只更新相机；实例拥有全部 GL 资源，dispose 后可释放。 */
export class FullViewRenderer {
  readonly gl: WebGL2RenderingContext;
  readonly textureBytes: number;
  readonly gpuTimes: number[] = [];
  private readonly program: WebGLProgram;
  private readonly textures: WebGLTexture[] = [];
  private readonly vao: WebGLVertexArrayObject;
  private readonly uniforms = new Map<string, WebGLUniformLocation>();
  private readonly timer: TimerExtension | null;
  private readonly queries: WebGLQuery[] = [];
  private readonly size: Volume["size"];
  private readonly origin: Volume["origin"];
  private readonly top: number;

  constructor(canvas: HTMLCanvasElement, volume: Volume) {
    const gl = canvas.getContext("webgl2", { antialias: false, alpha: false, depth: false });
    if (!gl) throw new Error("当前浏览器无法创建 WebGL 2 上下文");
    this.gl = gl;
    this.size = volume.size;
    this.origin = volume.origin;
    this.top = volume.levels.length - 1;
    this.timer = gl.getExtension("EXT_disjoint_timer_query_webgl2") as TimerExtension | null;
    const program = gl.createProgram()!;
    for (const [type, source] of [[gl.VERTEX_SHADER, vertexShader], [gl.FRAGMENT_SHADER, fragmentShader]] as const) {
      const shader = gl.createShader(type)!;
      gl.shaderSource(shader, source);
      gl.compileShader(shader);
      if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(shader)!);
      gl.attachShader(program, shader);
      gl.deleteShader(shader);
    }
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(program)!);
    this.program = program;
    this.vao = gl.createVertexArray()!;
    gl.useProgram(program);
    for (const name of ["uDirectory", "uBricks", "uPalette", "uGroups", "uSize", "uOrigin", "uRight", "uUp", "uDirection", "uResolution", "uSpan", "uFar", "uTop", "uDebug"]) {
      this.uniforms.set(name, gl.getUniformLocation(program, name)!);
    }
    gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
    const max3D = gl.getParameter(gl.MAX_3D_TEXTURE_SIZE) as number;
    if (volume.levels[0].size.some(n => n > max3D)) throw new Error(`目录超过设备的三维纹理边长 ${max3D}`);
    const materialRows = Math.ceil(volume.groups.length / 256);
    let bytes = materialRows * 256 * 28;
    const directory = gl.createTexture()!;
    this.textures.push(directory);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_3D, directory);
    gl.texStorage3D(gl.TEXTURE_3D, volume.levels.length, gl.R32UI, ...volume.levels[0].size);
    volume.levels.forEach((level, index) => {
      gl.texSubImage3D(gl.TEXTURE_3D, index, 0, 0, 0, ...level.size, gl.RED_INTEGER, gl.UNSIGNED_INT, level.cells);
      bytes += level.cells.byteLength;
    });
    gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_MIN_FILTER, gl.NEAREST_MIPMAP_NEAREST);
    gl.texParameteri(gl.TEXTURE_3D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.uniform1i(this.uniforms.get("uDirectory")!, 0);

    // 砖池按 8 MiB 分页，避免单张二维纹理的边长限制；尾页补齐也计入预算。
    const pageCells = 2048 * 2048;
    const pages = Math.max(1, Math.ceil(volume.bricks.length / pageCells));
    bytes += pages * pageCells * 2;
    if (pages > gl.getParameter(gl.MAX_ARRAY_TEXTURE_LAYERS) || bytes >= 2 ** 31) throw new Error("体素数据超过设备纹理层数或 2 GiB 数据预算");
    const bricks = gl.createTexture()!;
    this.textures.push(bricks);
    gl.activeTexture(gl.TEXTURE1);
    gl.bindTexture(gl.TEXTURE_2D_ARRAY, bricks);
    gl.texStorage3D(gl.TEXTURE_2D_ARRAY, 1, gl.R16UI, 2048, 2048, pages);
    for (let page = 0; page < pages; page++) {
      const data = volume.bricks.subarray(page * pageCells, (page + 1) * pageCells);
      const fullRows = Math.floor(data.length / 2048);
      if (fullRows) gl.texSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, 0, page, 2048, fullRows, 1, gl.RED_INTEGER, gl.UNSIGNED_SHORT, data.subarray(0, fullRows * 2048));
      if (data.length % 2048) gl.texSubImage3D(gl.TEXTURE_2D_ARRAY, 0, 0, fullRows, page, data.length % 2048, 1, 1, gl.RED_INTEGER, gl.UNSIGNED_SHORT, data.subarray(fullRows * 2048));
    }
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.uniform1i(this.uniforms.get("uBricks")!, 1);

    const palette = gl.createTexture()!;
    this.textures.push(palette);
    gl.activeTexture(gl.TEXTURE2);
    gl.bindTexture(gl.TEXTURE_2D, palette);
    gl.texStorage2D(gl.TEXTURE_2D, 1, gl.RGBA8, 1536, materialRows);
    const faces = new Uint8Array(materialRows * 256 * 24);
    faces.set(volume.palette);
    gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, 1536, materialRows, gl.RGBA, gl.UNSIGNED_BYTE, faces);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.uniform1i(this.uniforms.get("uPalette")!, 2);
    const groups = gl.createTexture()!;
    this.textures.push(groups);
    gl.activeTexture(gl.TEXTURE3);
    gl.bindTexture(gl.TEXTURE_2D, groups);
    gl.texStorage2D(gl.TEXTURE_2D, 1, gl.R32UI, 256, materialRows);
    const groupIDs = new Uint32Array(materialRows * 256);
    groupIDs.set(volume.groups);
    gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, 256, materialRows, gl.RED_INTEGER, gl.UNSIGNED_INT, groupIDs);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.uniform1i(this.uniforms.get("uGroups")!, 3);
    this.textureBytes = bytes;
    const error = gl.getError();
    if (error !== gl.NO_ERROR) throw new Error(`上传体素纹理失败，WebGL 错误 ${error}`);
  }

  /** 绘制准确体素采样；hierarchy=false 只用于对比分层跳跃的收益。 */
  draw(camera: Camera, options: { hierarchy?: boolean; debug?: number; timing?: boolean } = {}): void {
    const gl = this.gl, timer = this.timer;
    if (timer && gl.getParameter(timer.GPU_DISJOINT_EXT)) {
      for (const q of this.queries) gl.deleteQuery(q);
      this.queries.length = 0;
      this.gpuTimes.length = 0;
    }
    while (this.queries.length && gl.getQueryParameter(this.queries[0], gl.QUERY_RESULT_AVAILABLE)) {
      const query = this.queries.shift()!;
      this.gpuTimes.push((gl.getQueryParameter(query, gl.QUERY_RESULT) as number) / 1e6);
      if (this.gpuTimes.length > 240) this.gpuTimes.shift();
      gl.deleteQuery(query);
    }
    const query = timer && options.timing !== false && this.queries.length < 8 ? gl.createQuery() : null;
    if (query) gl.beginQuery(timer!.TIME_ELAPSED_EXT, query);
    const { right, up, direction } = cameraBasis(camera);
    // 射线从世界包围盒前方一格出发，避免无意义的长距离相减损失浮点精度。
    const depth = this.size.reduce((sum,c,i)=>sum+Math.abs(direction[i])*c,0);
    const distance = camera.target.reduce((sum,c,i)=>sum+(c-this.origin[i]-this.size[i]/2)*direction[i],0)+depth/2+1;
    const origin = camera.target.map((c, i) => c - this.origin[i] - direction[i] * distance);
    gl.viewport(0, 0, gl.drawingBufferWidth, gl.drawingBufferHeight);
    gl.clearColor(0.055, 0.075, 0.095, 1);
    if (options.debug === 1) gl.clearColor(0, 0, 0, 1);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.useProgram(this.program);
    gl.bindVertexArray(this.vao);
    gl.uniform3fv(this.uniforms.get("uSize")!, this.size);
    gl.uniform3fv(this.uniforms.get("uOrigin")!, origin);
    gl.uniform3fv(this.uniforms.get("uRight")!, right);
    gl.uniform3fv(this.uniforms.get("uUp")!, up);
    gl.uniform3fv(this.uniforms.get("uDirection")!, direction);
    gl.uniform2f(this.uniforms.get("uResolution")!, gl.drawingBufferWidth, gl.drawingBufferHeight);
    gl.uniform1f(this.uniforms.get("uSpan")!, camera.span);
    gl.uniform1f(this.uniforms.get("uFar")!, depth + 2);
    gl.uniform1i(this.uniforms.get("uTop")!, options.hierarchy === false ? 0 : this.top);
    gl.uniform1i(this.uniforms.get("uDebug")!, options.debug ?? 0);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
    if (query) { gl.endQuery(timer!.TIME_ELAPSED_EXT); this.queries.push(query); }
  }

  /** 丢弃热身帧的查询，使测量只包含正式采样区间。 */
  resetTiming(): void {
    for (const query of this.queries) this.gl.deleteQuery(query);
    this.queries.length = 0;
    this.gpuTimes.length = 0;
  }

  /** 释放显存；加载另一份体积前调用，避免两套世界长期同时驻留。 */
  dispose(): void {
    for (const texture of this.textures) this.gl.deleteTexture(texture);
    for (const query of this.queries) this.gl.deleteQuery(query);
    this.gl.deleteVertexArray(this.vao);
    this.gl.deleteProgram(this.program);
  }
}
