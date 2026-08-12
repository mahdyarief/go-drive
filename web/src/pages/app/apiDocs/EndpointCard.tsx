import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { CodeBlock } from './CodeBlock'

interface EndpointCardProps {
  baseUrl: string
}

export function EndpointCard({ baseUrl }: EndpointCardProps) {
  const { t } = useTranslation()

  const responseExample = `{
  "data": {
    "files": [
      { "name": "photo.jpg", "id": "<file_id>", "size": 204800 }
    ],
    "failed": []
  }
}`

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex flex-wrap items-center gap-2 text-base">
          <Badge className="font-mono">POST</Badge>
          <code className="font-mono text-sm">{baseUrl}/api/v1/uploads</code>
        </CardTitle>
        <CardDescription>{t('apiDocs.endpointDescription')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <div className="space-y-1">
          <p className="font-medium">{t('apiDocs.endpointAuthLabel')}</p>
          <p className="text-muted-foreground">{t('apiDocs.endpointAuth')}</p>
        </div>
        <div className="space-y-1">
          <p className="font-medium">{t('apiDocs.endpointRequestLabel')}</p>
          <p className="text-muted-foreground">{t('apiDocs.endpointRequestFormat')}</p>
          <ul className="list-disc space-y-1 pl-4 text-muted-foreground">
            <li>{t('apiDocs.endpointFieldFiles')}</li>
            <li>{t('apiDocs.endpointFieldFolderId')}</li>
          </ul>
        </div>
        <div className="space-y-2">
          <p className="font-medium">{t('apiDocs.endpointResponseLabel')}</p>
          <p className="text-muted-foreground">{t('apiDocs.endpointResponseFormat')}</p>
          <CodeBlock code={responseExample} language={t('apiDocs.jsonLabel')} />
        </div>
      </CardContent>
    </Card>
  )
}
