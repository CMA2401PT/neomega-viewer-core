export const vertexShader = `#version 300 es
void main() {
  vec2 p = vec2((gl_VertexID << 1) & 2, gl_VertexID & 2);
  gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0);
}`;

export const orthographicRayShader = `
// 像素坐标相对于视口，单像素拾取直接提供一条世界射线。
void cameraRay() {
  vec2 pixel=((gl_FragCoord.xy-uViewport.xy)/uViewport.zw-0.5)
    *vec2(uViewport.z/uViewport.w,1.0)*uSpan;
  rayOrigin=uOrigin+uRight*pixel.x+uUp*pixel.y;
  rayDirection=uDirection;
}`;

const declarations = `
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
uniform vec3 uSize, uOrigin, uRight, uUp, uDirection, uBackground;
uniform vec4 uViewport;
uniform float uSpan, uFar, uClipY;
uniform int uTop, uDebug;
uniform bool uOpenBoundary;
vec3 rayOrigin, rayDirection, inverseDirection;
int face=1, visits=0;
ivec3 voxelCell, previousCell;
uint previous=0u, firstMaterial=0u;
int firstSide=0;
ivec3 firstCell=ivec3(0), missingCell=ivec3(0);
float firstDepth=0.0;
bool touched=false, suppressEntry=false;
int result=0;
vec3 accumulated=vec3(0);
float transmission=1.0;
vec3 toLinear(vec3 c) {return mix(c/12.92,pow((c+0.055)/1.055,vec3(2.4)),greaterThan(c,vec3(0.04045)));}
vec3 toSRGB(vec3 c) {return mix(c*12.92,1.055*pow(c,vec3(1.0/2.4))-0.055,greaterThan(c,vec3(0.0031308)));}
vec4 surface(uint id,int side) {return texelFetch(uPalette,ivec2(int(id&255u)*6+side,int(id>>8)),0);}
uint group(uint id) {return texelFetch(uGroups,ivec2(int(id&255u),int(id>>8)),0).r;}
`;

export const traversalShader = `
// 遍历只报告材质边界；颜色、拾取和深度由独立的输出部分决定。
float leaveCell(vec3 lower,float side) {
  vec3 edge=lower+mix(vec3(0),vec3(side),greaterThan(rayDirection,vec3(0)));
  vec3 distances=(edge-rayOrigin)*inverseDirection;
  for(int a=0;a<3;a++) if(abs(rayDirection[a])<1e-8) distances[a]=1e30;
  face=distances.x<=distances.y && distances.x<=distances.z ? 0 : distances.y<=distances.z ? 1 : 2;
  float nextTime=distances[face];
  ivec3 nextCell=ivec3(floor(rayOrigin+rayDirection*nextTime));
  for(int a=0;a<3;a++) {
    if(distances[a]==nextTime) nextCell[a]=int(lower[a])+(rayDirection[a]>0.0 ? int(side) : -1);
    else nextCell[a]=clamp(nextCell[a],int(lower[a]),int(lower[a]+side)-1);
  }
  previousCell=clamp(nextCell,ivec3(lower),ivec3(lower+side)-1);
  voxelCell=nextCell;
  return nextTime;
}
uint brickColor(uint brick,ivec3 cell) {
  uint index=(brick-65536u)*512u+uint(cell.x+8*(cell.y+8*cell.z));
  return texelFetch(uBricks,ivec3(int(index&2047u),int((index>>11)&2047u),int(index>>22)),0).r;
}
bool crossBoundary(uint current,float t) {
  if(suppressEntry) {previous=current;suppressEntry=false;return false;}
  if(current==previous)return false;
  if(previous!=0u && current!=0u && group(previous)!=0u && group(previous)==group(current)) {previous=current;return false;}
  int outgoing=face*2+(rayDirection[face]>0.0 ? 1 : 0);
  if(previous!=0u && visitSurface(previous,outgoing,t,previousCell))return true;
  previous=current;
  if(current!=0u && visitSurface(current,outgoing^1,t,voxelCell))return true;
  return false;
}
void traceRay() {
  vec3 bounds=vec3(uSize.x,min(uSize.y,uClipY),uSize.z);
  if(bounds.y<=0.0)return;
  inverseDirection=1.0/mix(vec3(1e-30),rayDirection,greaterThan(abs(rayDirection),vec3(1e-8)));
  vec3 enter=-rayOrigin*inverseDirection;
  vec3 leave=(bounds-rayOrigin)*inverseDirection;
  vec3 nearAxis=min(enter,leave),farAxis=max(enter,leave);
  for(int a=0;a<3;a++) if(abs(rayDirection[a])<1e-8) {
    if(rayOrigin[a]<0.0 || rayOrigin[a]>=bounds[a])return;
    nearAxis[a]=-1e30;farAxis[a]=1e30;
  }
  face=nearAxis.x>=nearAxis.y && nearAxis.x>=nearAxis.z ? 0 : nearAxis.y>=nearAxis.z ? 1 : 2;
  float t=max(0.0,nearAxis[face]);
  int exitFace=farAxis.x<=farAxis.y && farAxis.x<=farAxis.z ? 0 : farAxis.y<=farAxis.z ? 1 : 2;
  float end=farAxis[exitFace];
  if(t>=end)return;
  // 水平窗口边界是数据裁切，Y 切片则有意显示切开的实心表面。
  suppressEntry=t==0.0 || (uOpenBoundary && face!=1);
  voxelCell=clamp(ivec3(floor(rayOrigin+rayDirection*t)),ivec3(0),ivec3(bounds)-1);
  int level=uTop;
  for(int iteration=0;iteration<16384;iteration++) {
    if(t>=end || any(lessThan(voxelCell,ivec3(0))) || any(greaterThanEqual(vec3(voxelCell),bounds))) {
      face=exitFace;
      previousCell=clamp(ivec3(floor(rayOrigin+rayDirection*end)),ivec3(0),ivec3(bounds)-1);
      if(uOpenBoundary && exitFace!=1) {
        result=2;
        missingCell=ivec3(floor(rayOrigin+rayDirection*end));
        missingCell[exitFace]=rayDirection[exitFace]>0.0 ? int(bounds[exitFace]) : -1;
      } else crossBoundary(0u,end);
      return;
    }
    visits++;
    float side=8.0*exp2(float(level));
    ivec3 cell=clamp(voxelCell>>(3+level),ivec3(0),textureSize(uDirectory,level)-1);
    uint value=texelFetch(uDirectory,cell,level).r;
    if(value==0xfffffffeu) {result=2;missingCell=voxelCell;return;}
    if(level>0 && value==0xffffffffu) {level--;continue;}
    if(value>=65536u) {
      for(int step=0;step<25;step++) {
        ivec3 voxel=voxelCell;
        if(t>=end || any(notEqual(voxel>>3,cell)))break;
        visits++;
        if(crossBoundary(brickColor(value,voxel&7),t))return;
        t=leaveCell(vec3(voxel),1.0);
      }
    } else {
      if(crossBoundary(value,t))return;
      t=leaveCell(vec3(cell)*side,side);
    }
    level=min(uTop,level+1);
  }
  result=3;
}
`;

export const colorOutputShader = `
out vec4 outColor;
bool visitSurface(uint id,int side,float t,ivec3 cell) {
  vec4 rgba=surface(id,side);
  if(rgba.a==0.0)return false;
  if(!touched) {touched=true;firstDepth=t;firstMaterial=id;firstSide=side;firstCell=cell;}
  if(uDebug==1)return true;
  float light=side/2==1 ? 1.0 : side/2==0 ? 0.82 : 0.65;
  accumulated+=transmission*rgba.a*toLinear(rgba.rgb*light);
  transmission*=1.0-rgba.a;
  return transmission==0.0;
}
void outputPixel() {
  if(result==3) {outColor=vec4(1,0,1,1);return;}
  if(uDebug==1) {
    outColor=touched ? vec4(float(firstMaterial&255u)/255.0,float(firstMaterial>>8)/255.0,float(firstSide)/255.0,1) : vec4(0,0,0,1);
    return;
  }
  if(uDebug==2) {float heat=float(visits)/160.0;outColor=vec4(heat,heat*heat,0.2,1);return;}
  vec3 background=uBackground;
  if(result==2) {
    float stripe=mod(floor((gl_FragCoord.x+gl_FragCoord.y)/8.0),2.0);
    background=mix(vec3(0.15,0.14,0.12),vec3(0.24,0.21,0.16),stripe);
  }
  outColor=vec4(toSRGB(accumulated+transmission*toLinear(background)),1);
  gl_FragDepth=touched ? firstDepth/uFar : 1.0;
}
`;

const pickOutputShader = `
layout(location=0) out ivec4 outHit;
bool visitSurface(uint id,int side,float t,ivec3 cell) {
  if(surface(id,side).a==0.0)return false;
  touched=true;firstCell=cell;firstSide=side;return true;
}
void outputPixel() {
  outHit=touched ? ivec4(firstCell,firstSide+1) : result==2 ? ivec4(missingCell,-1) : ivec4(0,0,0,result==3 ? -2 : 0);
}
`;

export function makeFragmentShader(picking = false): string {
  // 替换射线生成与输出即可接入其他相机或合成方式，层级遍历保持共用。
  return `#version 300 es\n${declarations}\n${orthographicRayShader}\n${picking ? pickOutputShader : colorOutputShader}\n${traversalShader}\nvoid main(){cameraRay();traceRay();outputPixel();}`;
}
export const fragmentShader = makeFragmentShader();
