export const vertexShader = `#version 300 es
void main() {
  vec2 p = vec2((gl_VertexID << 1) & 2, gl_VertexID & 2);
  gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0);
}`;

// 目录纹理的 mip 是占用层次，不是颜色平均；同色实心区域可直接返回准确交点。
export const fragmentShader = `#version 300 es
precision highp float;
precision highp int;
precision highp usampler3D;
precision highp usampler2DArray;
precision highp sampler2D;
uniform usampler3D uDirectory;
uniform usampler2DArray uBricks;
uniform sampler2D uPalette;
uniform vec3 uSize, uOrigin, uRight, uUp, uDirection;
uniform vec2 uResolution;
uniform float uSpan, uFar;
uniform int uTop, uDebug;
out vec4 outColor;

vec3 inverseDirection;
vec3 rayOrigin;
int face = 1;
int visits = 0;
ivec3 voxelCell;

float leaveCell(vec3 lower, float side) {
  vec3 edge = lower + mix(vec3(0.0), vec3(side), greaterThan(uDirection, vec3(0.0)));
  vec3 distances = (edge - rayOrigin) * inverseDirection;
  for (int a=0; a<3; a++) if (abs(uDirection[a]) < 1e-8) distances[a]=1e30;
  face = distances.x <= distances.y && distances.x <= distances.z ? 0 : distances.y <= distances.z ? 1 : 2;
  float nextTime = distances[face];
  ivec3 nextCell = ivec3(floor(rayOrigin + uDirection * nextTime));
  // 跨界轴由整数边界决定，其余轴限制在当前区域内，避免浮点消减使射线停在原格。
  for (int a=0; a<3; a++) {
    if (distances[a] == nextTime) nextCell[a] = int(lower[a]) + (uDirection[a] > 0.0 ? int(side) : -1);
    else nextCell[a] = clamp(nextCell[a], int(lower[a]), int(lower[a] + side) - 1);
  }
  voxelCell = nextCell;
  return nextTime;
}

uint brickColor(uint brick, ivec3 cell) {
  uint index = (brick-256u)*512u + uint(cell.x + 8*(cell.y + 8*cell.z));
  return texelFetch(uBricks, ivec3(int(index & 2047u), int((index >> 11) & 2047u), int(index >> 22)), 0).r;
}

void finishHit(uint color, float t) {
  gl_FragDepth=t/uFar;
  if (uDebug == 1) { outColor=vec4(float(color)/255.0, float(face)/255.0, 0, 1); return; }
  if (uDebug == 2) { float heat=float(visits)/160.0; outColor=vec4(heat,heat*heat,0.2,1); return; }
  vec3 rgb=texelFetch(uPalette,ivec2(int(color),0),0).rgb;
  float light=face==1 ? 1.0 : face==0 ? 0.82 : 0.65;
  outColor=vec4(rgb*light,1);
}

void main() {
  vec2 pixel=(gl_FragCoord.xy/uResolution-0.5)*vec2(uResolution.x/uResolution.y,1.0)*uSpan;
  rayOrigin=uOrigin+uRight*pixel.x+uUp*pixel.y;
  inverseDirection=1.0/mix(vec3(1e-30),uDirection,greaterThan(abs(uDirection),vec3(1e-8)));
  vec3 enter=(vec3(0)-rayOrigin)*inverseDirection;
  vec3 leave=(uSize-rayOrigin)*inverseDirection;
  vec3 nearAxis=min(enter,leave), farAxis=max(enter,leave);
  for(int a=0;a<3;a++) {
    if(abs(uDirection[a])<1e-8) {
      if(rayOrigin[a]<0.0 || rayOrigin[a]>=uSize[a]) discard;
      nearAxis[a]=-1e30;farAxis[a]=1e30;
    }
  }
  face=nearAxis.x>=nearAxis.y && nearAxis.x>=nearAxis.z ? 0 : nearAxis.y>=nearAxis.z ? 1 : 2;
  float t=max(0.0,nearAxis[face]);
  float end=min(farAxis.x,min(farAxis.y,farAxis.z));
  if(t>=end) discard;
  voxelCell=clamp(ivec3(floor(rayOrigin+uDirection*t)),ivec3(0),ivec3(uSize)-1);
  int level=uTop;
  for(int iteration=0;iteration<16384;iteration++) {
    if(t>=end || any(lessThan(voxelCell,ivec3(0))) || any(greaterThanEqual(voxelCell,ivec3(uSize)))) discard;
    visits++;
    float side=8.0*exp2(float(level));
    ivec3 cell=voxelCell >> (3+level);
    // 顶层在短轴上只有一个目录项，包围盒先限制了有效射线范围。
    cell=clamp(cell,ivec3(0),textureSize(uDirectory,level)-1);
    uint value=texelFetch(uDirectory,cell,level).r;
    if(level>0 && value==0xffffffffu) {level--;continue;}
    if(value>0u && value<256u) {finishHit(value,t);return;}
    vec3 lower=vec3(cell)*side;
    if(value>=256u) {
      // 只有混合小块需要逐格遍历；8³ 小块中的射线最多跨过 22 个格子。
      ivec3 brickCell=cell;
      for(int step=0;step<25;step++) {
        ivec3 voxel=voxelCell;
        if(any(notEqual(voxel >> 3,brickCell))) break;
        visits++;
        uint color=brickColor(value,voxel & 7);
        if(color!=0u) {finishHit(color,t);return;}
        t=leaveCell(vec3(voxel),1.0);
      }
    } else {
      t=leaveCell(lower,side);
    }
    // 跨出当前区域后尝试粗一级；混合区域再下降，不必每一步都从树根开始。
    level=min(uTop,level+1);
  }
  // 超出遍历上限是可见错误，不把未完成的射线冒充背景。
  outColor=vec4(1,0,1,1);
}`;
