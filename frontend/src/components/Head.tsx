/**
 * The bar every screen starts with.
 *
 * There were six of these, each with its own height, padding and title size,
 * which is why the app read as two applications stitched together once the
 * workspace arrived. One bar, one density, one place to change it.
 */
export default function Head({
  title,
  count,
  children,
}: {
  title: string
  /** Shown small beside the title when the screen counts something. */
  count?: number | string
  /** Controls, pushed to the trailing edge. */
  children?: React.ReactNode
}) {
  return (
    <header className="no-drag flex h-bar flex-none items-center gap-3 border-b border-line/70 px-4">
      <h1 className="flex items-baseline gap-2 whitespace-nowrap text-[12px] font-medium">
        {title}
        {count !== undefined && (
          <span className="text-[10px] tabular-nums text-faint">{count}</span>
        )}
      </h1>
      {children && <div className="ml-auto flex min-w-0 items-center gap-0.5">{children}</div>}
    </header>
  )
}

/**
 * A control in a screen bar: quiet until wanted, and lit while it is on.
 * Shared for the same reason the bar is — there were four of these too.
 */
export function Verb({
  on,
  onClick,
  Icon,
  children,
}: {
  on?: boolean
  onClick: () => void
  Icon: React.ComponentType<{ size?: number }>
  children?: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-1.5 whitespace-nowrap rounded-md px-2 py-1 text-[10.5px] transition-colors ${
        on ? "bg-raised text-text" : "text-faint hover:bg-raised hover:text-text"
      }`}
    >
      <Icon size={12} />
      {children}
    </button>
  )
}
