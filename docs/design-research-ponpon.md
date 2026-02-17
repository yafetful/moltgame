# Ponpon Mania 设计调研报告

> 调研日期: 2026-02-12
> 目标: 分析 ponpon-mania.com 的视觉风格、动画技术，为 moltgame 前端改造提供参考

## 一、网站概览

- **URL**: https://ponpon-mania.com/
- **类型**: Interactive Comic (交互式漫画)
- **作者**: Justine Soulié (插画) + Patrick Heng (开发)
- **语言**: EN / FR 双语

## 二、技术栈

| 技术 | 用途 |
|------|------|
| **Nuxt.js** (Vue) | 框架，SSR/SPA |
| **WebGL2** (自定义 shader) | 漫画阅读器 — canvas class `webgl-canvas`，非 Three.js/PixiJS/OGL |
| **CSS Transitions** (193 个元素) | 页面交互微动画 |
| **CSS Transform** (35 个元素) | 角色/元素视差、缩放 |
| **SVG** (23 个) | 图标、装饰元素 |
| **Canvas** (1 个, 2436×1342) | WebGL2 漫画渲染器 |
| **Cloudflare** | CDN + Analytics |
| **Google Analytics** | G-KL9SR46655 |

### 不使用的技术
- 无 GSAP / ScrollTrigger
- 无 Three.js / PixiJS / OGL
- 无 Lottie
- 无 Locomotive Scroll / Lenis / smooth-scrollbar
- 无 Framer Motion (Vue 生态)

## 三、字体

| 字体 | 用途 |
|------|------|
| **Libre Franklin** | 正文、导航 |
| 自定义粗体衬线 | Logo "ponpon mania"（手绘风格） |
| 超粗黑体 (Extra Bold) | About 页大标题 |

## 四、配色体系

| 色彩 | Hex 近似 | Tailwind 近似 | 用途 |
|------|---------|--------------|------|
| 薰衣草紫 | `#7B68EE` | violet-400 | 首页背景主色 |
| 暖橙/琥珀 | `#F5A623` | amber-500 | 太阳、强调色、Chapters/About 背景 |
| 粉红 | `#F9A8D4` | pink-300 | 云朵、装饰、角色高亮 |
| 红色 | `#EF4444` | red-500 | 按钮、警告、能量感 |
| 翠绿 | `#22C55E` | green-500 | 鳄鱼角色、Chapter 1 封面 |
| 米白 | `#FAF9F5` | — | 漫画纸张纹理、内页底色 |
| 深灰/黑 | `#171717` | neutral-900 | 404 页、CTA 按钮、描边 |

### 配色规律
- 每个页面一个主色调 (首页=紫, Chapters=橙, About=橙→粉, 404=深灰)
- 角色配色丰富但不超过 3-4 色/角色
- 高饱和 + 暖色系为主，冷色仅用于建筑/背景层

## 五、页面结构与设计语言

### 5.1 首页 (/)
- 全屏沉浸式，`overflow: hidden`，无传统滚动
- 大面积薰衣草紫背景 + 橙色圆形太阳
- 三个主角角色 (羊/狼/鳄鱼) 占据画面中心
- 粉色云朵层叠在底部
- 中央 CTA: "read now" 黑色胶囊按钮
- 导航: "chapters" / logo / "about" + 语言切换
- 右上角: 旋转黑胶唱片徽章 "YOUR INTERACTIVE COMIC"
- 角色有微妙的浮动/呼吸动画 (CSS transform)
- 气球缓慢漂浮

### 5.2 Chapters 页 (/chapters)
- 橙色同心圆背景 (橙→深橙渐变环)
- 漫画封面设计成黑胶唱片封面 (可横向切换)
- 底部音频播放器 (章节标题 + 播放/上一首/下一首)
- 右下 "support us" 按钮

### 5.3 About 页 (/about)
- 全屏分段滚动 (snap-like)，右侧导航点
- Section 1: 米白底 + 角色散落铺满 + 居中大标题
- Section 2 (#ponpon): 粉色底 + 角色降落伞从天而降 + 文字逐词弹入
- Section 3 (#team): 橙色底 + "behind the legend" + 角色脑袋内两人工作
- 文字进场: 逐词动画，大胆的排版节奏

### 5.4 漫画章节页 (/chapter/1)
- body overflow: hidden (全屏)
- 横向滚动 (scroll-to-advance) 推进漫画页
- WebGL2 Canvas (2436×1342) 渲染漫画格
- 漫画格: 4 格/页 (2×2)，纸质纹理背景
- 底部进度条 + 章节标题
- 右侧全屏切换按钮
- 背景: 橙+粉色块不规则形状

### 5.5 404 页
- 深灰底 + 米白角色 (墨镜羊 dab 姿势)
- 思维气泡 "LOOSER"
- "Oopsy, page not found" + 回首页按钮
- 连错误页都有角色表演和叙事性

## 六、角色设计特征

### 三大主角
| 角色 | 动物 | 配色 | 个性配饰 | 性格 |
|------|------|------|---------|------|
| Ponpon | 羊 | 米白/粉耳 | 星形墨镜、项链 | 自大狂、想当 DJ |
| Jean-Loup | 狼 | 灰紫 | 魔术师帽、饮品 | 狂野酒保 |
| Simon | 鳄鱼 | 翠绿 | 西装领、手机、餐碟 | 社交美食博主 |

### 角色绘画风格
- **描边**: 粗黑线 (2-3px)，统一线宽
- **填色**: 平面填色为主，极少阴影/渐变
- **体型**: 圆润、肥胖、SD 身材比例 (大头小身)
- **表情**: 夸张 — 大嘴笑、眯眼、得意、愤怒，情绪一眼可辨
- **风格融合**: 法式 BD (Bande Dessinée) × 日式 SD (Super Deformed)
- **个性化**: 每个角色通过配饰 (墨镜/帽子/手机) 建立辨识度
- **一致性**: 所有角色共享同一描边粗细、同一色彩饱和度、同一身体比例

## 七、动画手法

| 动画类型 | 实现方式 | 效果 |
|---------|---------|------|
| 首页角色浮动 | CSS transform + transition | 缓慢上下浮动，呼吸感 |
| 气球漂浮 | CSS animation | 缓慢漂移 + 轻微旋转 |
| 页面转场 | 自定义 (可能 JS transition) | 全屏遮罩滑入/推出 |
| About 文字弹入 | IntersectionObserver + CSS | 逐词/逐行从下方弹入 |
| About 角色进场 | Scroll-driven | 降落伞从上方降落、角色旋转散落 |
| 漫画翻页 | WebGL2 custom shader | 纸张翻页质感、视差深度 |
| 导航 hover | CSS transition | 缩放 + 底部出现圆角框 |
| 唱片旋转 | CSS animation rotate | 持续旋转的黑胶唱片徽章 |

## 八、对 moltgame 的适用性建议

### 直接可借鉴
| Ponpon 特征 | moltgame 借鉴方向 |
|------------|-----------------|
| 圆润卡通动物角色 | Agent 形象用卡通动物，配饰代表游戏风格/等级 |
| 高饱和暖色调 (紫+橙+粉) | 竞技场"热闹派对"氛围，比冷色科技风更亲切 |
| 粗描边平面插画 | 适合 AI 批量生成，风格一致性好 |
| 漫画格分镜布局 | 观战回放关键时刻以漫画分格展示 (摊牌/投票) |
| 全屏沉浸式无滚动条 | 游戏观战页面 |
| 角色在所有状态页面出现 | 空队列、加载中、错误页都有角色互动 |
| 音频集成 | 对局可加背景音效 |

### 需要调整的
| Ponpon 特征 | moltgame 需要的调整 |
|------------|-------------------|
| 法式 BD 风格 | 更偏日系二次元 (anime/manga) |
| 3 个固定角色 | 需要大量不同角色 (每个 agent 独特) |
| 横向漫画阅读 | 观战是实时状态展示，非漫画阅读 |
| WebGL 漫画渲染 | 不需要 WebGL，React + CSS 动画足够 |
| Nuxt.js (Vue) | 我们用 Next.js (React) |

### 推荐配色方案 (基于 Ponpon 调整)

```
主色: 薰衣草紫 #7B68EE (平台背景)
强调: 琥珀橙 #F5A623 (Chakra、奖金、高亮)
辅助: 粉色 #F9A8D4 (装饰、柔和元素)
游戏-扑克: 翡翠绿 #059669 (牌桌传统绿)
游戏-狼人杀: 靛蓝 #4338CA (夜晚氛围)
危险/紧张: 红色 #EF4444
文字/描边: 深灰 #171717
纸张/卡片: 米白 #FAF9F5
```

## 九、风格探索记录

### 已生成方案

| 方案 | 文件 | 评价 |
|------|------|------|
| Style A: Ponpon Chibi | `nanobanana-output/style-a-ponpon-chibi.png` | 粗描边+高饱和，稍偏儿童简笔画 |
| Style B: 复古街机 | `nanobanana-output/style-b-retro-arcade.png` | 霓虹+深色底，电竞感强但抠图难 |
| Style C: 极简 Kawaii | `nanobanana-output/style-c-kawaii-minimal.png` | 柔和渐变，太文艺清新缺竞技感 |

### 最终选定: Korean Webtoon 平涂风格

经过 7 轮迭代，最终选定 **Korean Webtoon 平涂风格**：
- 平面赛璐珞着色 + 锐利硬边阴影
- 干净细线稿 + 大色块 + 极少纹理
- 高对比 (深色背景 vs 明亮主体)
- 图形化简洁美学

| 轮次 | 风格 | 文件前缀 | 结果 |
|------|------|---------|------|
| 1 | Ponpon Chibi | style-a | 太像儿童简笔画 |
| 2 | 日系/Gacha/高端游戏 | style-d/e/f | 方向各有优缺 |
| 3 | Nago 猫 Q版 | agent-* | 可爱但想看更多选项 |
| 4 | 波斯王子 / 纯二次元 | pop-* / anime-* | 各有特色但不完全匹配 |
| 5 | 法式卡通 | cartoon-* | 接近但不够精致 |
| 6 | Manhwa 现代动漫 | manhwa-* | 渐变太柔和 |
| 7 | **Webtoon 平涂** | **webtoon-*** | **最终选定 ✅** |

详细提示词模板和生成参数见: `.claude/projects/.../memory/art-style-guide.md`
