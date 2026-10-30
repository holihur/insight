import type { SVGProps } from "react"

/** A circular labyrinth mark inspired by Westworld's title sequence. */
export function MazeIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      viewBox="0 0 48 48"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.35"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    >
      <circle cx="24" cy="24" r="19.5" opacity="0.35" />
      <path d="M24 4.5A19.5 19.5 0 1 1 4.5 24" />
      <path d="M24 10.5A13.5 13.5 0 1 0 37.5 24" />
      <path d="M24 17A7 7 0 1 1 17 24" />
      <path d="M24 4.5v6" opacity="0.7" />
      <path d="M37.5 24h6" opacity="0.7" />
      <path d="M4.5 24h6" opacity="0.7" />
      <path d="M10.5 10.5l4.4 4.4" opacity="0.5" />
      <path d="M33.1 33.1l4.4 4.4" opacity="0.5" />
      <path d="M33.1 14.9l4.4-4.4" opacity="0.35" />
      <circle cx="24" cy="24" r="2" fill="currentColor" stroke="none" />
    </svg>
  )
}
