interface LinkTokenProps {
  token: string
}

export function LinkToken({ token }: LinkTokenProps) {
  return <span className="font-mono text-xs text-muted-foreground truncate">{token}</span>
}
