# Kensan Logo Brand Guidelines

## 1. ロゴコンセプト

### 1.1 込めた思い

Kensanのロゴは、**「日々の積み重ねで自分を磨き続けるエンジニア」** を視覚的に表現しています。

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│    ✦ スパークル                                          │
│      磨き上がった輝き、成果の結晶                         │
│                                                         │
│         ╱╲                                              │
│        ╱  ╲  成長曲線                                   │
│       ╱    ╲ 右肩上がりの継続的な成長                    │
│      ╱      ╲                                           │
│     ╱────────╲                                          │
│    ┌──────────┐                                         │
│    │  砥石    │ 土台となる基盤、プラットフォーム          │
│    └──────────┘                                         │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 1.2 各要素の意味

| 要素 | 視覚表現 | 込めた意味 |
|------|----------|------------|
| **成長曲線** | 右肩上がりのカーブ | 時間を投資することで着実に成長していく軌跡。急がず、でも確実に上へ向かう |
| **スパークル** | 4点の星型（大1、小2） | 研鑽によって磨き上がった輝き。努力の結晶、スキルの光 |
| **砥石** | 下部の長方形 + ハイライト | 成長の土台。インフラ、プラットフォーム、日々の習慣。ハイライトは「研ぐ面」を強調 |

### 1.3 名前との関係

**研鑽（けんさん）** = 学問や技芸を深く究めること

- 「研」: 研ぐ、磨く → 砥石のモチーフ
- 「鑽」: 深く究める → 成長曲線で表現
- 輝き: 研鑽の結果 → スパークルで表現

---

## 2. ロゴファイル一覧

```
kensan-logo/
└── svg/
    ├── kensan-logo-light.svg      # メインロゴ（ライトモード用）
    ├── kensan-logo-dark.svg       # メインロゴ（ダークモード用）
    ├── kensan-favicon-light.svg   # ファビコン用（ライト）
    ├── kensan-favicon-dark.svg    # ファビコン用（ダーク）
    ├── kensan-logo-mono-black.svg # モノクロ（黒）
    └── kensan-logo-mono-white.svg # モノクロ（白）
```

### 2.1 使い分け

| ファイル | 用途 |
|----------|------|
| `kensan-logo-light.svg` | 白・明るい背景での表示 |
| `kensan-logo-dark.svg` | 黒・暗い背景での表示 |
| `kensan-favicon-*.svg` | ファビコン、小サイズ表示（32px以下） |
| `kensan-logo-mono-*.svg` | 単色印刷、透かし、特殊用途 |

---

## 3. カラーパレット

### 3.1 プライマリカラー

| 名前 | HEX | RGB | 用途 |
|------|-----|-----|------|
| **Primary** | `#0EA5E9` | rgb(14, 165, 233) | 成長曲線メイン、ブランドカラー |
| **Primary Light** | `#38BDF8` | rgb(56, 189, 248) | スパークル、ダークモードの曲線 |
| **Highlight** | `#7DD3FC` | rgb(125, 211, 252) | 曲線のハイライト、サブスパークル |
| **Light** | `#BAE6FD` | rgb(186, 230, 253) | 最も薄いアクセント |

### 3.2 ニュートラルカラー（砥石）

| 名前 | HEX | RGB | 用途 |
|------|-----|-----|------|
| **Stone** | `#64748B` | rgb(100, 116, 139) | 砥石（ライトモード） |
| **Stone Light** | `#94A3B8` | rgb(148, 163, 184) | 砥石ハイライト |
| **Stone Dark** | `#475569` | rgb(71, 85, 105) | 砥石（ダークモード） |

### 3.3 CSS変数

```css
:root {
  /* Primary Colors */
  --kensan-primary: #0EA5E9;
  --kensan-primary-light: #38BDF8;
  --kensan-highlight: #7DD3FC;
  --kensan-light: #BAE6FD;
  --kensan-lightest: #E0F2FE;
  
  /* Stone Colors */
  --kensan-stone: #64748B;
  --kensan-stone-light: #94A3B8;
  --kensan-stone-dark: #475569;
  
  /* Text Colors */
  --kensan-text-on-light: #0c4a6e;
  --kensan-text-on-dark: #e0f2fe;
}
```

---

## 4. サイズ・余白規定

### 4.1 最小サイズ

| 用途 | 最小サイズ | 備考 |
|------|------------|------|
| デジタル表示 | 24px | これ以下はファビコン版を使用 |
| ファビコン | 16px | 簡略化版を使用 |
| 印刷 | 10mm | モノクロ版推奨 |

### 4.2 クリアスペース（余白）

ロゴの周囲には、ロゴ高さの **20%** 以上の余白を確保してください。

```
┌────────────────────────────┐
│                            │
│   ┌────────────────────┐   │
│   │                    │   │
│   │       LOGO         │   │  ← 上下左右に20%の余白
│   │                    │   │
│   └────────────────────┘   │
│                            │
└────────────────────────────┘
```

---

## 5. 使用ガイドライン

### 5.1 推奨される使い方

✅ **OK**

- 指定されたカラーパレットでの使用
- 十分なコントラストのある背景での使用
- アスペクト比を維持した拡大・縮小
- ダークモードでは `kensan-logo-dark.svg` を使用

### 5.2 禁止事項

❌ **NG**

- ロゴの色を勝手に変更する
- ロゴを回転させる
- ロゴを引き伸ばす（アスペクト比を崩す）
- 複雑な背景の上に直接配置する
- ロゴに影やエフェクトを追加する
- ロゴの一部だけを使用する（スパークルだけ等）

---

## 6. フロントエンド実装ガイド

### 6.1 Reactコンポーネント例

```tsx
// components/KensanLogo.tsx
import { useTheme } from 'next-themes';

interface KensanLogoProps {
  size?: number;
  className?: string;
}

export const KensanLogo = ({ size = 40, className }: KensanLogoProps) => {
  const { theme } = useTheme();
  const isDark = theme === 'dark';

  return (
    <svg
      viewBox="0 0 100 100"
      width={size}
      height={size}
      className={className}
      aria-label="Kensan Logo"
    >
      {/* 砥石 */}
      <rect
        x="15" y="61"
        width="70" height="17"
        rx="2.5"
        fill={isDark ? '#475569' : '#64748B'}
      />
      <line
        x1="17" y1="63.5"
        x2="83" y2="63.5"
        stroke={isDark ? '#64748B' : '#94A3B8'}
        strokeWidth="1.5"
        strokeLinecap="round"
      />
      
      {/* 成長曲線 */}
      <path
        d="M18,58 Q38,50 55,38 Q72,24 82,12 L82,58 Z"
        fill={isDark ? '#38BDF8' : '#0EA5E9'}
      />
      <path
        d="M22,56 Q40,48 56,37 Q72,24 80,14 L80,22 Q70,32 54,44 Q38,54 22,58 Z"
        fill={isDark ? '#BAE6FD' : '#7DD3FC'}
        opacity={isDark ? 0.6 : 0.7}
      />
      
      {/* スパークル */}
      <path
        d="M88,8 L90,3 L92,8 L97,10 L92,12 L90,17 L88,12 L83,10 Z"
        fill={isDark ? '#7DD3FC' : '#38BDF8'}
      />
      <path
        d="M78,2 L79,-1 L80,2 L83,3 L80,4 L79,7 L78,4 L75,3 Z"
        fill={isDark ? '#BAE6FD' : '#7DD3FC'}
      />
      <path
        d="M95,20 L96,18 L97,20 L99,21 L97,22 L96,24 L95,22 L93,21 Z"
        fill={isDark ? '#E0F2FE' : '#BAE6FD'}
      />
    </svg>
  );
};
```

### 6.2 SVGファイルをインポートして使用

```tsx
// Next.js + SVGR の場合
import KensanLogoLight from '@/assets/kensan-logo-light.svg';
import KensanLogoDark from '@/assets/kensan-logo-dark.svg';

const Logo = () => {
  const { theme } = useTheme();
  const LogoComponent = theme === 'dark' ? KensanLogoDark : KensanLogoLight;
  
  return <LogoComponent className="w-10 h-10" />;
};
```

### 6.3 ファビコン設定（Next.js）

```tsx
// app/layout.tsx
export const metadata = {
  icons: {
    icon: [
      { url: '/favicon-32.png', sizes: '32x32', type: 'image/png' },
      { url: '/favicon-16.png', sizes: '16x16', type: 'image/png' },
    ],
    apple: '/apple-touch-icon.png',
  },
};
```

### 6.4 Tailwind CSS との組み合わせ

```tsx
// ヘッダーでの使用例
<header className="flex items-center gap-3">
  <KensanLogo size={32} />
  <span className="text-xl font-semibold text-kensan-text">
    Kensan
  </span>
</header>
```

```js
// tailwind.config.js
module.exports = {
  theme: {
    extend: {
      colors: {
        kensan: {
          primary: '#0EA5E9',
          'primary-light': '#38BDF8',
          highlight: '#7DD3FC',
          light: '#BAE6FD',
          stone: '#64748B',
          'stone-dark': '#475569',
          text: '#0c4a6e',
          'text-dark': '#e0f2fe',
        },
      },
    },
  },
};
```

---

## 7. 応用例

### 7.1 ローディングアニメーション

スパークルを点滅させることで「磨いている」感じを演出できます。

```css
@keyframes sparkle {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.kensan-logo .sparkle {
  animation: sparkle 1.5s ease-in-out infinite;
}

.kensan-logo .sparkle:nth-child(2) {
  animation-delay: 0.3s;
}

.kensan-logo .sparkle:nth-child(3) {
  animation-delay: 0.6s;
}
```

### 7.2 ホバーエフェクト

```css
.kensan-logo {
  transition: transform 0.2s ease;
}

.kensan-logo:hover {
  transform: scale(1.05);
}

.kensan-logo:hover .sparkle {
  filter: brightness(1.2);
}
```

---

## 8. 更新履歴

| バージョン | 日付 | 内容 |
|------------|------|------|
| 1.0 | 2025-01-22 | 初版作成 |

---

*Kensan Brand Guidelines v1.0*
*Created with 研鑽 (Kensan) spirit 🔷✨*
