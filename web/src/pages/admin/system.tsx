import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { adminApi } from '@/lib/api'
import type { SystemInfo } from '@/lib/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Cpu, HardDrive, MemoryStick, Server, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h < 24) return `${h}h ${m}m`
  const d = Math.floor(h / 24)
  return `${d}d ${h % 24}h ${m}m`
}

function formatGB(gb: number): string {
  return `${gb.toFixed(1)} GB`
}

function MetricBar({
  label,
  value,
  detail,
  icon: Icon,
}: {
  label: string
  value: number
  detail: string
  icon: typeof Cpu
}) {
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between text-sm">
        <span className="flex items-center gap-2 text-muted-foreground">
          <Icon className="h-4 w-4" />
          {label}
        </span>
        <span className="tabular-nums font-medium">{detail}</span>
      </div>
      <div className="h-2 w-full rounded-full bg-muted">
        <div
          className="h-2 rounded-full bg-primary"
          style={{ width: `${Math.min(Math.max(value, 0), 100)}%` }}
        />
      </div>
    </div>
  )
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="tabular-nums font-medium truncate">{value}</span>
    </div>
  )
}

export default function SystemPage() {
  const { t } = useTranslation()

  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey: ['admin', 'system'],
    queryFn: () => adminApi<SystemInfo>('/api/admin/system'),
    refetchInterval: 5000,
  })

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-muted-foreground">{t('app.loading')}</p>
      </div>
    )
  }

  if (isError || !data) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-muted-foreground">{t('admin.loadFailed')}</p>
      </div>
    )
  }

  const s = data

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('admin.system')}</h1>
          <p className="text-sm text-muted-foreground">{t('admin.systemDesc')}</p>
        </div>
        <Button variant="outline" size="icon" onClick={() => refetch()} disabled={isFetching} aria-label="Refresh">
          <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Server className="h-5 w-5" />
            {t('admin.systemOverview')}
          </CardTitle>
        </CardHeader>
        <Separator />
        <CardContent className="pt-4 grid grid-cols-1 md:grid-cols-2 gap-x-10 gap-y-4">
          <div className="space-y-4">
            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {t('admin.systemHost')}
            </p>
            <MetricBar
              label="CPU"
              value={s.cpu_pct}
              detail={`${s.cpu_pct.toFixed(1)}%`}
              icon={Cpu}
            />
            <MetricBar
              label="Memory"
              value={s.mem_pct}
              detail={`${s.mem_used_mb} / ${s.mem_total_mb} MB`}
              icon={MemoryStick}
            />
            <MetricBar
              label="Disk"
              value={s.disk_pct}
              detail={`${formatGB(s.disk_used_gb)} / ${formatGB(s.disk_total_gb)}`}
              icon={HardDrive}
            />
          </div>
          <div className="space-y-4">
            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {t('admin.systemProcess')} (PID {s.pid})
            </p>
            <MetricBar
              label="CPU"
              value={s.proc_cpu_pct}
              detail={`${s.proc_cpu_pct.toFixed(1)}%`}
              icon={Cpu}
            />
            <MetricBar
              label="RSS"
              value={s.mem_total_mb > 0 ? (s.proc_rss_mb / s.mem_total_mb) * 100 : 0}
              detail={`${s.proc_rss_mb.toFixed(0)} MB`}
              icon={MemoryStick}
            />
            <DetailRow label={t('admin.systemUptime')} value={formatUptime(s.uptime_s)} />
            <DetailRow label={t('admin.systemGoroutines')} value={String(s.goroutines)} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <HardDrive className="h-5 w-5" />
            {t('admin.systemStorage')}
          </CardTitle>
        </CardHeader>
        <Separator />
        <CardContent className="pt-4 space-y-4">
          <MetricBar
            label="Disk"
            value={s.disk_pct}
            detail={`${s.disk_pct.toFixed(1)}% used`}
            icon={HardDrive}
          />
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <DetailRow label={t('admin.systemDiskTotal')} value={formatGB(s.disk_total_gb)} />
            <DetailRow label={t('admin.systemDiskUsed')} value={formatGB(s.disk_used_gb)} />
            <DetailRow label={t('admin.systemDiskFree')} value={formatGB(s.disk_free_gb)} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Server className="h-5 w-5" />
            {t('admin.systemHostInfo')}
          </CardTitle>
        </CardHeader>
        <Separator />
        <CardContent className="pt-4">
          <div className="grid grid-cols-2 gap-x-8 gap-y-3">
            <DetailRow label={t('admin.systemHostname')} value={s.host || '—'} />
            <DetailRow label={t('admin.systemOS')} value={s.os || '—'} />
            <DetailRow label={t('admin.systemArch')} value={s.arch || '—'} />
            <DetailRow label={t('admin.systemCores')} value={String(s.cpu_count)} />
            <DetailRow label={t('admin.systemUptime')} value={formatUptime(s.uptime_s)} />
            <DetailRow label={t('admin.systemHeapAlloc')} value={`${s.heap_alloc_mb.toFixed(1)} MB`} />
            <DetailRow label={t('admin.systemThreads')} value={String(s.proc_threads)} />
            <DetailRow label={t('admin.systemFDs')} value={String(s.proc_fds)} />
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
