import * as THREE from "three";

// 这些数值只决定本例的操作手感，与区块边长无关。
const MOVE_SPEED: number = 16; // 世界方块单位/秒。
const LOOK_SENSITIVITY: number = 0.003; // 弧度/鼠标像素。
const MAX_PITCH: number = 1.55; // 略小于 π/2，防止视线翻过头顶。

/** 拖动调整视线，WASD 只沿水平面移动，空格和 Shift 控制升降。 */
export class FlightControls {
  private readonly keys = new Set<string>();
  private readonly direction = new THREE.Vector3();
  private readonly rotation = new THREE.Euler(-0.35, 0, 0, "YXZ");
  private readonly camera: THREE.PerspectiveCamera;

  constructor(camera: THREE.PerspectiveCamera, canvas: HTMLCanvasElement) {
    this.camera = camera;
    camera.quaternion.setFromEuler(this.rotation);
    // 键盘事件绑定在画布上，需要让画布可获得焦点。
    canvas.tabIndex = 0;
    const movementKeys = new Set(["KeyW", "KeyA", "KeyS", "KeyD", "Space", "ShiftLeft", "ShiftRight"]);
    canvas.addEventListener("keydown", event => {
      if (movementKeys.has(event.code)) {
        event.preventDefault();
        this.keys.add(event.code);
      }
    });
    canvas.addEventListener("keyup", event => this.keys.delete(event.code));
    canvas.addEventListener("blur", () => this.keys.clear());
    canvas.addEventListener("pointerdown", event => {
      canvas.focus();
      canvas.setPointerCapture(event.pointerId);
    });
    canvas.addEventListener("pointermove", event => {
      if (!canvas.hasPointerCapture(event.pointerId)) return;
      this.rotation.y -= event.movementX * LOOK_SENSITIVITY;
      // 避免翻过头顶，水平移动方向始终由偏航角决定。
      this.rotation.x = THREE.MathUtils.clamp(
        this.rotation.x - event.movementY * LOOK_SENSITIVITY,
        -MAX_PITCH,
        MAX_PITCH,
      );
      camera.quaternion.setFromEuler(this.rotation);
    });
    canvas.addEventListener("contextmenu", event => event.preventDefault());
    canvas.focus();
  }

  /** 使用秒数计算位移，斜向移动与直行保持相同速度。 */
  update(seconds: number): void {
    const forward: number = Number(this.keys.has("KeyW")) - Number(this.keys.has("KeyS"));
    const right: number = Number(this.keys.has("KeyD")) - Number(this.keys.has("KeyA"));
    const up: number = Number(this.keys.has("Space")) - Number(this.keys.has("ShiftLeft") || this.keys.has("ShiftRight"));
    // Three.js 相机默认朝 -Z；只取偏航角，不让抬头低头改变 WASD 的高度。
    const yaw: number = this.rotation.y;
    this.direction.set(
      right * Math.cos(yaw) - forward * Math.sin(yaw),
      up,
      -right * Math.sin(yaw) - forward * Math.cos(yaw),
    ).normalize();
    this.camera.position.addScaledVector(this.direction, seconds * MOVE_SPEED);
  }
}
