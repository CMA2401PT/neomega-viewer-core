export const vertexShader = `#version 300 es
void main() {
  vec2 p = vec2((gl_VertexID << 1) & 2, gl_VertexID & 2);
  gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0);
}`;

// 目录纹理合并相同材质；同材质透明区域整体跨过，表面混合只发生在材质边界。
export const fragmentShader = `#version 300 es
precision highp float;
precision highp int;
precision highp usampler3D;
precision highp usampler2D;
precision highp usampler2DArray;
precision highp sampler2D;
uniform usampler3D uDirectory;
uniform usampler2DArray uBricks;
uniform sampler2D uPalette;
uniform usampler2D uGroups;
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
  uint index = (brick-65536u)*512u + uint(cell.x + 8*(cell.y + 8*cell.z));
  return texelFetch(uBricks, ivec3(int(index & 2047u), int((index >> 11) & 2047u), int(index >> 22)), 0).r;
}

vec3 accumulated=vec3(0);
float transmission=1.0;
uint previous=0u;
float firstDepth=0.0;
bool touched=false;

vec3 toLinear(vec3 c) {return mix(c/12.92,pow((c+0.055)/1.055,vec3(2.4)),greaterThan(c,vec3(0.04045)));}
vec3 toSRGB(vec3 c) {return mix(c*12.92,1.055*pow(c,vec3(1.0/2.4))-0.055,greaterThan(c,vec3(0.0031308)));}
vec4 surface(uint id,int side) {return texelFetch(uPalette,ivec2(int(id&255u)*6+side,int(id>>8)),0);}
uint group(uint id) {return texelFetch(uGroups,ivec2(int(id&255u),int(id>>8)),0).r;}

bool addSurface(uint id,int side,float t) {
  vec4 rgba=surface(id,side);
  if(rgba.a==0.0)return false;
  if(!touched){firstDepth=t;touched=true;}
  if(uDebug==1){outColor=vec4(float(id&255u)/255.0,float(id>>8)/255.0,float(side)/255.0,1);gl_FragDepth=t/uFar;return true;}
  float light=side/2==1 ? 1.0 : side/2==0 ? 0.82 : 0.65;
  accumulated+=transmission*rgba.a*toLinear(rgba.rgb*light);
  transmission*=1.0-rgba.a;
  return transmission==0.0;
}

// 同一透明介质的接缝不计色；不同介质接触时，先退出旧材质，再进入新材质。
bool crossBoundary(uint current,float t) {
  if(current==previous)return false;
  if(previous!=0u && current!=0u && group(previous)!=0u && group(previous)==group(current)) {previous=current;return false;}
  int outgoing=face*2+(uDirection[face]>0.0 ? 1 : 0);
  if(previous!=0u && addSurface(previous,outgoing,t))return true;
  previous=current;
  if(current!=0u && addSurface(current,outgoing^1,t))return true;
  return false;
}

void finishPixel() {
  if(uDebug==1){if(!touched)outColor=vec4(0,0,0,1);return;}
  if(uDebug==2){float heat=float(visits)/160.0;outColor=vec4(heat,heat*heat,0.2,1);return;}
  outColor=vec4(toSRGB(accumulated+transmission*toLinear(vec3(0.055,0.075,0.095))),1);
  gl_FragDepth=firstDepth/uFar;
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
  int exitFace=farAxis.x<=farAxis.y && farAxis.x<=farAxis.z ? 0 : farAxis.y<=farAxis.z ? 1 : 2;
  float end=farAxis[exitFace];
  if(t>=end) discard;
  voxelCell=clamp(ivec3(floor(rayOrigin+uDirection*t)),ivec3(0),ivec3(uSize)-1);
  int level=uTop;
  for(int iteration=0;iteration<16384;iteration++) {
    if(t>=end || any(lessThan(voxelCell,ivec3(0))) || any(greaterThanEqual(voxelCell,ivec3(uSize)))) {face=exitFace;crossBoundary(0u,end);finishPixel();return;}
    visits++;
    float side=8.0*exp2(float(level));
    ivec3 cell=voxelCell >> (3+level);
    // 顶层在短轴上只有一个目录项，包围盒先限制了有效射线范围。
    cell=clamp(cell,ivec3(0),textureSize(uDirectory,level)-1);
    uint value=texelFetch(uDirectory,cell,level).r;
    if(level>0 && value==0xffffffffu) {level--;continue;}

    vec3 lower=vec3(cell)*side;
    if(value>=65536u) {
      // 只有混合小块需要逐格遍历；8³ 小块中的射线最多跨过 22 个格子。
      ivec3 brickCell=cell;
      for(int step=0;step<25;step++) {
        ivec3 voxel=voxelCell;
        if(any(notEqual(voxel >> 3,brickCell))) break;
        visits++;
        uint color=brickColor(value,voxel & 7);
        if(crossBoundary(color,t)) {finishPixel();return;}
        t=leaveCell(vec3(voxel),1.0);
      }
    } else {
      if(crossBoundary(value,t)){finishPixel();return;}
      t=leaveCell(lower,side);
    }
    // 跨出当前区域后尝试粗一级；混合区域再下降，不必每一步都从树根开始。
    level=min(uTop,level+1);
  }
  // 超出遍历上限是可见错误，不把未完成的射线冒充背景。
  outColor=vec4(1,0,1,1);
}`;
