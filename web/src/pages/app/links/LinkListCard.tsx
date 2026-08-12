import type { ReactNode } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Link2 } from 'lucide-react'

interface LinkListCardProps {
  title: string
  emptyLabel: string
  isEmpty: boolean
  children: ReactNode
}

export function LinkListCard({ title, emptyLabel, isEmpty, children }: LinkListCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Link2 className="h-4 w-4" />
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {isEmpty ? <p className="text-sm text-muted-foreground">{emptyLabel}</p> : children}
      </CardContent>
    </Card>
  )
}
