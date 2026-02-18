interface KensanLogoProps {
  size?: number
  className?: string
}

export function KensanLogo({ size = 32, className }: KensanLogoProps) {
  return (
    <svg
      viewBox="0 0 100 100"
      width={size}
      height={size}
      className={className}
      aria-label="Kensan Logo"
    >
      {/* 砥石（Whetstone） */}
      <rect x="15" y="61" width="70" height="17" rx="2.5" fill="#64748B" />
      <line
        x1="17"
        y1="63.5"
        x2="83"
        y2="63.5"
        stroke="#94A3B8"
        strokeWidth="1.5"
        strokeLinecap="round"
      />

      {/* 成長曲線（Growth Curve） */}
      <path d="M18,58 Q38,50 55,38 Q72,24 82,12 L82,58 Z" fill="#0EA5E9" />
      <path
        d="M22,56 Q40,48 56,37 Q72,24 80,14 L80,22 Q70,32 54,44 Q38,54 22,58 Z"
        fill="#7DD3FC"
        opacity="0.7"
      />

      {/* スパークル（Sparkle） */}
      <path
        d="M88,8 L90,3 L92,8 L97,10 L92,12 L90,17 L88,12 L83,10 Z"
        fill="#38BDF8"
      />
      <path
        d="M78,2 L79,-1 L80,2 L83,3 L80,4 L79,7 L78,4 L75,3 Z"
        fill="#7DD3FC"
      />
      <path
        d="M95,20 L96,18 L97,20 L99,21 L97,22 L96,24 L95,22 L93,21 Z"
        fill="#BAE6FD"
      />
    </svg>
  )
}
