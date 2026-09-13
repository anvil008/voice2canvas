import { useId, type SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement>;

function IconBase({ children, ...props }: IconProps) {
  return (
    <svg
      aria-hidden="true"
      fill="none"
      height="20"
      viewBox="0 0 24 24"
      width="20"
      {...props}
    >
      {children}
    </svg>
  );
}

export function MicIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <rect x="9" y="3" width="6" height="11" rx="3" stroke="currentColor" strokeWidth="1.8" />
      <path d="M6.5 11.5a5.5 5.5 0 0 0 11 0M12 17v4M9 21h6" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </IconBase>
  );
}

export function MutedIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <rect x="9" y="3" width="6" height="11" rx="3" stroke="currentColor" strokeWidth="1.8" />
      <path d="m4 4 16 16M6.5 11.5a5.5 5.5 0 0 0 8.8 4.4M12 17v4M9 21h6" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </IconBase>
  );
}

export function StopIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <rect x="6.5" y="6.5" width="11" height="11" rx="2" fill="currentColor" />
    </IconBase>
  );
}

export function SunIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <circle cx="12" cy="12" r="3.5" stroke="currentColor" strokeWidth="1.7" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42M17.65 17.65l1.42 1.42M2 12h2M20 12h2M4.93 19.07l1.42-1.42M17.65 6.35l1.42-1.42" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
    </IconBase>
  );
}

export function MoonIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M20 15.2A8.5 8.5 0 0 1 8.8 4a8.5 8.5 0 1 0 11.2 11.2Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.7" />
    </IconBase>
  );
}

export function ChevronIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="m8 10 4 4 4-4" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
    </IconBase>
  );
}

export function SparkIcon(props: IconProps) {
  const gradientId = useId().replaceAll(":", "");
  return (
    <IconBase {...props}>
      <defs>
        <linearGradient id={gradientId} x1="3" y1="3" x2="21" y2="21" gradientUnits="userSpaceOnUse">
          <stop stopColor="var(--grad-blue, #217bfe)" />
          <stop offset=".46" stopColor="var(--grad-cyan, #078efb)" />
          <stop offset="1" stopColor="var(--grad-violet, #ac87eb)" />
        </linearGradient>
      </defs>
      <path d="M12 2.8c.6 4.8 2.4 6.6 7.2 7.2-4.8.6-6.6 2.4-7.2 7.2-.6-4.8-2.4-6.6-7.2-7.2 4.8-.6 6.6-2.4 7.2-7.2Z" fill={`url(#${gradientId})`} />
      <path d="M18.5 16.5c.2 1.6.9 2.3 2.5 2.5-1.6.2-2.3.9-2.5 2.5-.2-1.6-.9-2.3-2.5-2.5 1.6-.2 2.3-.9 2.5-2.5Z" fill={`url(#${gradientId})`} />
    </IconBase>
  );
}

export function AgentsIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <circle cx="12" cy="7" r="2.6" stroke="currentColor" strokeWidth="1.7" />
      <circle cx="6" cy="16.5" r="2.2" stroke="currentColor" strokeWidth="1.7" />
      <circle cx="18" cy="16.5" r="2.2" stroke="currentColor" strokeWidth="1.7" />
      <path d="m10.6 9.2-3.1 5M13.4 9.2l3.1 5M8.3 16.5h7.4" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
    </IconBase>
  );
}

export function VoiceBackIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M5 10v4h3l4 3.5v-11L8 10H5Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.7" />
      <path d="M15 9.3a4 4 0 0 1 0 5.4M17.5 7a7 7 0 0 1 0 10" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
    </IconBase>
  );
}


export function CloseIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="m6 6 12 12M18 6 6 18" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </IconBase>
  );
}

export function ConnectIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path
        d="M9 3v5M15 3v5M12 22v-4M18 8v4a6 6 0 0 1-12 0V8a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1Z"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />
    </IconBase>
  );
}

export const PlugIcon = ConnectIcon;
