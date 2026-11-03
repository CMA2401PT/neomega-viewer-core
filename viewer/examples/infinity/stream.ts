import type { Vector3 } from "three";
import {
  decodeFrame,
  type SectionObject,
  type SectionPos,
  type SectionScene,
} from "@neomega/viewer/render";

// 应用的固定尺寸约定，与 world.go 的 cube.New(16) 一致，供坐标换算与 SectionScene 共用。
export const SECTION_SIZE: number = 16;
const MAX_PENDING_REQUESTS: number = 2;

/** 围绕相机加载有限范围的地面子区块，离开范围后释放几何并取消请求。 */
export class SectionLoader {
  readonly loaded = new Map<string, SectionObject>();
  error = "";
  private wanted = new Map<string, SectionPos>();
  private pending = new Map<string, AbortController>();
  private center = "";
  private readonly world: SectionScene;
  private readonly radius: number;

  constructor(world: SectionScene, radius: number = 3) {
    this.world = world;
    this.radius = radius;
  }

  /** 跨过区块边界时重排近处优先的目标，每帧补足两个请求名额。 */
  update(position: Vector3): void {
    // 1. 世界方块坐标换成区域坐标；floor 让负坐标也归入正确的区域。
    const x: number = Math.floor(position.x / SECTION_SIZE);
    const z: number = Math.floor(position.z / SECTION_SIZE);
    const center = `${x},${z}`;
    if (center !== this.center) {
      this.center = center;
      // 半径 3 是本例的性能取舍：最多驻留 (2×3+1)² = 49 个地面区域。
      const positions: SectionPos[] = [];
      for (let dx = -this.radius; dx <= this.radius; dx++) {
        for (let dz = -this.radius; dz <= this.radius; dz++) {
          positions.push({ x: x + dx, y: 0, z: z + dz });
        }
      }
      positions.sort((a, b) =>
        (a.x - x) ** 2 + (a.z - z) ** 2 - (b.x - x) ** 2 - (b.z - z) ** 2,
      );
      this.wanted = new Map(positions.map(pos => [`${pos.x},${pos.z}`, pos]));
      for (const [id, section] of this.loaded) {
        if (!this.wanted.has(id)) {
          // dispose 同时从 SectionScene 移除该区域，并释放它的几何资源。
          section.dispose();
          this.loaded.delete(id);
        }
      }
      for (const [id, controller] of this.pending) {
        if (!this.wanted.has(id)) controller.abort();
      }
    }
    // 2. 按距离补足请求名额；发生错误后显示原因，重新加载网页即可重试。
    if (this.error) return;
    for (const [id, pos] of this.wanted) {
      if (this.pending.size >= MAX_PENDING_REQUESTS) break;
      if (this.loaded.has(id) || this.pending.has(id)) continue;
      const controller = new AbortController();
      this.pending.set(id, controller);
      void this.load(id, pos, controller);
    }
  }

  private async load(id: string, pos: SectionPos, controller: AbortController): Promise<void> {
    try {
      const response: Response = await fetch(
        `/api/section?x=${pos.x}&z=${pos.z}`,
        { signal: controller.signal },
      );
      if (!response.ok) throw new Error(await response.text());
      const buffer: ArrayBuffer = await response.arrayBuffer();
      // 移动期间已经离开范围的响应不再解码、上传 GPU。
      controller.signal.throwIfAborted();
      // 3. prepare 创建区域资源，add 才将它接入场景；位置按 SECTION_SIZE 换成世界原点。
      const section: SectionObject = this.world.prepare(pos, decodeFrame(buffer));
      this.world.add(section);
      this.loaded.set(id, section);
    } catch (error: unknown) {
      if (!controller.signal.aborted) {
        this.error = String(error);
        console.error(error);
      }
    } finally {
      this.pending.delete(id);
    }
  }

  /** 页面关闭时取消请求并释放驻留区域；共享场景和贴图由 main.ts 释放。 */
  dispose(): void {
    for (const controller of this.pending.values()) controller.abort();
    for (const section of this.loaded.values()) section.dispose();
    this.loaded.clear();
  }
}
