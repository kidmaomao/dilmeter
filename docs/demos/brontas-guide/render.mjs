// Render deterministic frames exported by TestExportBrontasGuideDemo.
// No browser, live capture, input injection, or game data is used here.
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { Resvg } from '@resvg/resvg-js';
const require=createRequire(import.meta.url);
const { GIFEncoder, quantize, applyPalette }=require('gifenc');
const root=path.dirname(fileURLToPath(import.meta.url));
const input=path.resolve(process.argv[2] || path.join(root,'frames.json'));
const output=path.resolve(process.argv[3] || path.join(root,'brontas-guide-demo.gif'));
const demo=JSON.parse(await fs.readFile(input,'utf8'));
const W=1040,H=660;
const grid=demo.grid;
if(!grid)throw new Error('Regenerate frames with a fixed nine-cell grid');
for(const f of demo.frames)for(const r of f.rays||[]){
 if(r.offset===undefined)continue;
 const vertical=r.axis==='vertical',c=vertical?grid.xCenter:grid.yCenter,w=vertical?grid.xHalfWidth:grid.yHalfWidth;
 const slot=(r.offset-c)/(2*w);
 if(Math.abs(r.halfWidth-w)>1e-9 || Math.abs(slot-Math.round(slot))>1e-9 || Math.abs(Math.round(slot))>1)throw new Error('Ray outside the three contiguous grid lanes');
}
const cadence=demo.mode==='cadence';
const duration=(demo.durationMs || 16000)/1000;
const C={bg:'#070f17',panel:'#0e1b27',border:'#315269',text:'#ecf5fa',muted:'#abc0d0',faint:'#516c80',gold:'#ffbf47',red:'#ff4b5d',cyan:'#68eadb',blue:'#75bbff'};
const esc=x=>String(x).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
const text=(x,y,s,size=16,fill=C.text,weight=400,anchor='start')=>`<text x="${x}" y="${y}" fill="${fill}" font-size="${size}" font-weight="${weight}" text-anchor="${anchor}">${esc(s)}</text>`;
const rect=(x,y,w,h,fill,rx=0,extra='')=>`<rect x="${x}" y="${y}" width="${w}" height="${h}" rx="${rx}" fill="${fill}" ${extra}/>`;
const circle=(x,y,r,fill,extra='')=>`<circle cx="${x}" cy="${y}" r="${r}" fill="${fill}" ${extra}/>`;
const line=(x1,y1,x2,y2,stroke,width=1,extra='')=>`<path d="M${x1},${y1}L${x2},${y2}" fill="none" stroke="${stroke}" stroke-width="${width}" ${extra}/>`;
const mapPoint=p=>({x:292+p.x*42,y:195-p.y*42});
const hudPoint=p=>({x:696+p.x*86,y:311-p.y*86});
const cellIndex=p=>{const col=p.x<grid.xCenter-grid.xHalfWidth?0:p.x>grid.xCenter+grid.xHalfWidth?2:1;const row=p.y>grid.yCenter+grid.yHalfWidth?0:p.y<grid.yCenter-grid.yHalfWidth?2:1;return row*3+col+1;};
function gridLines(cx,cy,r){return [-1,1].map(k=>line(cx+(grid.xCenter+k*grid.xHalfWidth)*r,cy-r,cx+(grid.xCenter+k*grid.xHalfWidth)*r,cy+r,C.muted,1,'stroke-opacity=".35"')+line(cx-r,cy-(grid.yCenter+k*grid.yHalfWidth)*r,cx+r,cy-(grid.yCenter+k*grid.yHalfWidth)*r,C.muted,1,'stroke-opacity=".35"')).join('');}

function rayShapes(rays,now,cx,cy,radius,extent){
 return (rays||[]).filter(r=>r.offset!==undefined && r.halfWidth!==undefined && (!r.endsAtMs || now<r.endsAtMs+150)).map(r=>{
  const fill=now<r.lethalAtMs?C.gold:C.red, hw=r.halfWidth*radius;
  return r.axis==='vertical'
   ?rect(cx+r.offset*radius-hw,cy-extent,hw*2,extent*2,fill,0,`fill-opacity=".40" stroke="${fill}" stroke-width="1"`)
   :rect(cx-extent,cy-r.offset*radius-hw,extent*2,hw*2,fill,0,`fill-opacity=".40" stroke="${fill}" stroke-width="1"`);
 }).join('');
}

function arrow(a,b){
 const d=Math.hypot(b.x-a.x,b.y-a.y);if(d<1)return '';
 const dx=(b.x-a.x)/d,dy=(b.y-a.y)/d;
 return line(a.x,a.y,b.x,b.y,C.cyan,3.5)+`<path d="M${b.x-dx*13-dy*7},${b.y-dy*13+dx*7}L${b.x},${b.y}L${b.x-dx*13+dy*7},${b.y-dy*13-dx*7}" fill="none" stroke="${C.cyan}" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"/>`;
}

function frameSVG(frame){
 const {guide:v,atMs:now,step}=frame;
 const fresh=now<v.validUntilMs;
 const move=fresh && v.status==='move' && v.player && v.target;
 const elapsed=(now-demo.baseAtMs)/1000;
 let s=`<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}"><style>text{font-family:'Microsoft YaHei','Noto Sans CJK SC',sans-serif;font-variant-numeric:tabular-nums}</style><defs><clipPath id="arena"><circle cx="292" cy="346" r="202"/></clipPath><clipPath id="zone"><circle cx="696" cy="311" r="86"/></clipPath></defs>`;
 s+=rect(0,0,W,H,C.bg);
 s+=text(28,46,cadence?'二王 50% · 连续加速演示':'二王 50% · 走位提示动态演示',32,C.text,700);
 s+=text(1012,39,'演示数据 · 非实战录像',16,C.gold,500,'end');
 s+=text(1012,65,`${elapsed.toFixed(1)} / ${duration.toFixed(1)} s`,15,C.muted,400,'end');
 s+=text(30,76,cadence?'固定九宫格：纵向 3 列、横向 3 行，射线边界相接，无可站缝隙。':'人物由演示脚本移动；插件只提示，不控制角色。',16,C.muted);
 s+=rect(28,98,528,500,C.panel,16,`stroke="${C.border}"`);
 s+=text(50,130,step===0?'地图示意':'12 点落石区 · 放大',20,C.text,700)+text(532,130,step===0?'地图上方 = 12 点':'圆圈是落石区边界',14,C.muted,400,'end');
 s+=circle(292,346,202,'#122533',`stroke="${C.border}" stroke-width="2"`);
 if(step===0){
 s+=circle(292,346,148,'none',`stroke="${C.border}" stroke-dasharray="4 9"`);
 s+=line(90,346,494,346,C.border,1)+line(292,144,292,548,C.border,1);
 s+=`<g clip-path="url(#arena)">${rayShapes(frame.rays,now,292,195,42,400)}</g>`;
 for(const [clock,x,y,on] of [[12,292,195,true],[3,444,346,false],[6,292,497,false],[9,140,346,true]]){
  s+=circle(x,y,42,on?'#322d21':'#14232f',`fill-opacity=".78" stroke="${on?C.gold:C.faint}" stroke-width="${clock===12?3:1.5}" ${on?'':'stroke-dasharray="5 5"'}`);
  s+=text(x,y-49,`${clock}点`,16,on?C.gold:C.muted,600,'middle');
  if(on)s+=`<path d="M${x-10},${y+4}L${x-6},${y-11}L${x+1},${y-17}L${x+10},${y-4}L${x+8},${y+8}Z" fill="${C.gold}" fill-opacity=".7"/>`;
 }
 s+=circle(292,346,39,'#243e51',`stroke="${C.faint}" stroke-width="2"`);
 s+=text(292,342,'二王',20,C.text,700,'middle')+text(292,365,'50%',15,C.muted,500,'middle');
 const p=mapPoint(frame.player);
 s+=circle(p.x,p.y,12,C.blue,'fill-opacity=".13"')+circle(p.x,p.y,5.5,C.text,`stroke="${C.blue}" stroke-width="2"`);
 }else{
  s+=`<g clip-path="url(#arena)">${rayShapes(frame.rays,now,292,346,202,202)}`;
  s+=gridLines(292,346,202);
  {
   // Number the nine cells defined by the TWO EDGES of each wide band.
   let cell=1;
   for(const y of [.64,-.12,-.73])for(const x of [-.59,0,.59]){
    s+=text(292+x*202,346-y*202+5,String(cell++),16,C.muted,500,'middle');
   }
  }
  const p={x:292+frame.player.x*202,y:346-frame.player.y*202};
  s+=circle(p.x,p.y,.08*202,frame.contact?C.red:C.text,`stroke="${C.blue}" stroke-width="2"`);
  s+='</g>';
  s+=circle(292,346,202,'none',`stroke="${C.border}" stroke-width="2"`);
  s+=text(50,155,cadence?`第 ${frame.wave} 条 · 出线间隔 ${frame.gapMs?(frame.gapMs/1000).toFixed(2)+' s':'—'}`:step===3?'纵带约 40% 直径 · 横带约 51%':'宽度参与路径判断 · 不仅看中心线',14,C.muted);
 }
 s+=text(50,574,frame.caption,17,step===5?C.gold:C.text,600);

 s+=rect(580,98,432,500,'#09121b',16,`stroke="${C.border}" stroke-width="1.5"`);
 s+=text(602,136,'二王 50% · 走位提示',21,C.text,700);
 s+=rect(938,116,54,25,'#153042',5)+text(965,134,'演示',13,C.blue,500,'middle');
 s+=line(600,153,992,153,C.border);
 if(cadence){
 s+=text(604,185,`机制剩余 ${(frame.remainingMs/1000).toFixed(1)} s · 第 ${frame.wave} 条`,19,C.cyan,600);
 }else for(const [i,clock] of [12,3,6,9].entries()){
  const x=604+i*98,on=clock===12||clock===9;
  s+=text(x,185,`${clock}点 ${on?'◆':'—'}`,17,clock===12?C.cyan:on?C.gold:C.faint,600);
  if(clock===12)s+=line(x,194,x+66,194,C.cyan,2);
 }
 s+=circle(696,311,86,'#111f2a',`stroke="#65849b" stroke-width="2"`);
 s+=`<g clip-path="url(#zone)">${gridLines(696,311,86)}</g>`;
 if(fresh && v.player){
  s+=`<g clip-path="url(#zone)">${rayShapes(v.rays,now,696,311,86,86)}${gridLines(696,311,86)}`;
  if(move){const a=hudPoint(v.player),b=hudPoint(v.target);s+=arrow(a,b)+circle(b.x,b.y,6,C.cyan);}
  if(Math.hypot(v.player.x,v.player.y)<=1){const q=hudPoint(v.player);s+=circle(q.x,q.y,v.player.radius*86,C.text,`stroke="${C.blue}" stroke-width="2"`);}
  s+='</g>';
 }else{s+=text(696,318,'待定位',18,C.muted,500,'middle');}
 const deadline=!fresh||!v.lethalAtMs?'--':now>=v.lethalAtMs?'已致命':`${(Math.ceil((v.lethalAtMs-now)/100)/10).toFixed(1)} s`;
 s+=text(818,269,'最早射线致命',15,C.muted);
 s+=text(818,321,deadline,deadline==='已致命'?34:45,C.gold,700);
 s+=text(818,355,'12点区域',17,C.muted,500);
 const action=cadence && step===5?'演示结束':frame.contact?'示例判定命中 · 警惕风险':move?`移至第 ${cellIndex(v.target)} 格`:fresh?v.instruction:'数据过期 · 停止走位引导';
 const detail=frame.contact?'仅展示参数下的风险，不代表实战伤害判定':fresh?v.detail:'等待新的识别结果';
 s+=text(602,442,action,20,move?C.cyan:step===5?C.gold:C.text,700);
 s+=text(602,476,detail,14,C.muted);
 s+=line(600,509,992,509,C.border);
 s+=text(602,540,cadence?`预警 ${(frame.leadMs/1000).toFixed(2)} s（演示设定）`:'地图方向 · 对侧 6点',17,C.text,500);
 s+=text(602,569,cadence?'出线间隔 ≠ 致命延迟 · 尚未实战校准':step===3?'下方小角：本例容不下人物与余量':'宽度按参考图示意 · 待实测校准',14,C.muted);
 const labels=cadence?['等待入场','初段','加速','持续高频','归零待判','结束']:['入场','纵线','横线','多条射线','仅方向','数据中断'];
 for(let i=0;i<labels.length;i++){
  const x=28+i*164;
  s+=rect(x,612,154,30,i===step?'#183b49':'#101e29',6);
  s+=text(x+77,633,labels[i],14,i===step?C.cyan:C.muted,i===step?700:400,'middle');
 }
 s+=rect(28,651,984*Math.min(1,elapsed/duration),3,C.cyan);
 return s+'</svg>';
}

const fontFile=process.env.DILMETER_DEMO_FONT || (process.platform==='win32'?'C:/Windows/Fonts/msyh.ttc':undefined);
const options={font:{loadSystemFonts:!fontFile,fontFiles:fontFile?[fontFile]:[],defaultFontFamily:'Microsoft YaHei'},background:C.bg};
await fs.mkdir(path.dirname(output),{recursive:true});
const previewArg=process.argv.find(a=>a.startsWith('--preview='));
if(previewArg){
 const index=Number(previewArg.split('=')[1]);
 if(!Number.isInteger(index)||!demo.frames[index])throw new Error('Invalid preview frame');
 const previewPath=output.replace(/\.gif$/i,'.png');
 await fs.writeFile(previewPath,new Resvg(frameSVG(demo.frames[index]),options).render().asPng());
 console.log('Saved preview '+previewPath);
 process.exit(0);
}
// One palette for every frame eliminates palette flicker between phases.
const samples=[0,22,38,61,92,130,160,190].map(i=>new Resvg(frameSVG(demo.frames[i]),options).render().pixels);
const palette=quantize(Buffer.concat(samples),256,{format:'rgb565'});
const gif=GIFEncoder();
const captureFrames=new Set(cadence?[20,42,75,130,190,215,240]:[22,38,61,92,130,160]);
for(let i=0;i<demo.frames.length;i++){
 const svg=frameSVG(demo.frames[i]);
 const rendered=new Resvg(svg,options).render();
 const indexed=applyPalette(rendered.pixels,palette,'rgb565');
 gif.writeFrame(indexed,W,H,{palette,delay:80,repeat:0});
 if(captureFrames.has(i))await fs.writeFile(path.join(path.dirname(output),`brontas-frame-${String(i).padStart(3,'0')}.png`),rendered.asPng());
 if(i%50===0)console.log(`Rendered ${i}/${demo.frames.length}`);
}
gif.finish();
await fs.writeFile(output,gif.bytes());
console.log(`Saved ${output}: ${demo.frames.length} frames, ${duration} seconds.`);
