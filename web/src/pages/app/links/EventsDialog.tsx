import { useTranslation } from 'react-i18next'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { TrackedLinkEvent } from '@/lib/types'

interface EventsDialogProps {
  eventsFor: string | null
  events: TrackedLinkEvent[]
  onClose: () => void
}

export function EventsDialog({ eventsFor, events, onClose }: EventsDialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog open={!!eventsFor} onOpenChange={(o) => { if (!o) onClose() }}>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('links.events')}</DialogTitle>
          <DialogDescription>{t('links.tabTracked')}</DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          {events.length === 0 && <p className="text-sm text-muted-foreground">{t('links.noEvents')}</p>}
          {events.map((ev) => (
            <div key={ev.id} className="rounded-lg border p-3 text-sm">
              <div className="flex items-center justify-between">
                <span className="font-medium capitalize">{ev.event_type}</span>
                <span className="text-xs text-muted-foreground">{new Date(ev.timestamp).toLocaleString()}</span>
              </div>
              <p className="text-xs text-muted-foreground mt-1">
                {[ev.browser, ev.os, ev.country].filter(Boolean).join(' · ') || '—'}
              </p>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  )
}
