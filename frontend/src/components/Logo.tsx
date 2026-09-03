export function Logo({ size = 22, className = "" }: { size?: number; className?: string }) {
  return (
    <svg viewBox="0 0 1024 1024" width={size} height={size} className={className} aria-hidden="true">
      <rect width="1024" height="1024" rx="228" fill="#5b5bd6" />
      <g stroke="#fff" strokeWidth="72" strokeLinecap="round" strokeLinejoin="round" fill="none">
        <path d="M252 332h520M252 528l84 84 152-152M560 528h212M252 724h368" />
      </g>
    </svg>
  );
}
