import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { CodeBlock } from './apiDocs/CodeBlock'
import { EndpointCard } from './apiDocs/EndpointCard'

export default function ApiDocsPage() {
  const { t } = useTranslation()
  const baseUrl = window.location.origin

  const authExample = `Authorization: Bearer <your_api_key>`

  const curlExample = `curl -X POST ${baseUrl}/api/v1/uploads \\
  -H "Authorization: Bearer <your_api_key>" \\
  -F "files=@photo.jpg" \\
  -F "files=@document.pdf" \\
  -F "folderId=<uuid>"`

  const jsExample = `const formData = new FormData()
for (const file of fileInput.files) {
  formData.append('files', file)
}
if (folderId) {
  formData.append('folderId', folderId)
}

const res = await fetch('${baseUrl}/api/v1/uploads', {
  method: 'POST',
  headers: {
    Authorization: 'Bearer <your_api_key>',
  },
  body: formData,
})

const { data } = await res.json()
const uploadedFiles = data.files
const failedFiles = data.failed`

  const listKeysExample = `curl ${baseUrl}/api/t/api-keys \\
  -H "Authorization: Bearer <session_token>" \\
  -H "X-Org-Slug: <org_slug>"`

  const createKeyExample = `curl -X POST ${baseUrl}/api/t/api-keys \\
  -H "Authorization: Bearer <session_token>" \\
  -H "X-Org-Slug: <org_slug>" \\
  -H "Content-Type: application/json" \\
  -d '{"name": "<key_name>"}'`

  const deleteKeyExample = `curl -X DELETE ${baseUrl}/api/t/api-keys/<key_id> \\
  -H "Authorization: Bearer <session_token>" \\
  -H "X-Org-Slug: <org_slug>"`

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t('apiDocs.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('apiDocs.subtitle')}</p>
      </div>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">{t('apiDocs.overview')}</h2>
        <Card>
          <CardContent className="space-y-3 pt-6 text-sm">
            <p className="text-muted-foreground">{t('apiDocs.overviewBody1')}</p>
            <p className="text-muted-foreground">{t('apiDocs.overviewBody2')}</p>
            <p className="text-xs text-muted-foreground">{t('apiDocs.overviewBaseUrl', { baseUrl })}</p>
          </CardContent>
        </Card>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">{t('apiDocs.authentication')}</h2>
        <Card>
          <CardContent className="space-y-3 pt-6 text-sm">
            <p className="text-muted-foreground">{t('apiDocs.authBody')}</p>
            <CodeBlock code={authExample} language={t('apiDocs.httpLabel')} />
            <ul className="list-disc space-y-1 pl-4 text-muted-foreground">
              <li>{t('apiDocs.authPrefix')}</li>
              <li>{t('apiDocs.authShownOnce')}</li>
            </ul>
          </CardContent>
        </Card>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">{t('apiDocs.endpoints')}</h2>
        <EndpointCard baseUrl={baseUrl} />
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">{t('apiDocs.examples')}</h2>
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('apiDocs.curlExample')}</CardTitle>
            <CardDescription>{t('apiDocs.curlExampleNote')}</CardDescription>
          </CardHeader>
          <CardContent>
            <CodeBlock code={curlExample} language={t('apiDocs.bashLabel')} />
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('apiDocs.jsExample')}</CardTitle>
            <CardDescription>{t('apiDocs.jsExampleNote')}</CardDescription>
          </CardHeader>
          <CardContent>
            <CodeBlock code={jsExample} language={t('apiDocs.javascriptLabel')} />
          </CardContent>
        </Card>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">{t('apiDocs.errors')}</h2>
        <Card>
          <CardContent className="pt-6">
            <div className="overflow-x-auto rounded-md border">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b bg-muted text-left text-xs text-muted-foreground">
                    <th className="px-4 py-2 font-medium">{t('apiDocs.errorCode')}</th>
                    <th className="px-4 py-2 font-medium">{t('apiDocs.errorDescription')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  <tr>
                    <td className="px-4 py-2 font-mono text-xs">400</td>
                    <td className="px-4 py-2 text-muted-foreground">{t('apiDocs.error400Desc')}</td>
                  </tr>
                  <tr>
                    <td className="px-4 py-2 font-mono text-xs">401</td>
                    <td className="px-4 py-2 text-muted-foreground">{t('apiDocs.error401Desc')}</td>
                  </tr>
                  <tr>
                    <td className="px-4 py-2 font-mono text-xs">500</td>
                    <td className="px-4 py-2 text-muted-foreground">{t('apiDocs.error500Desc')}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">{t('apiDocs.apiKeys')}</h2>
        <Card>
          <CardContent className="space-y-4 pt-6 text-sm">
            <p className="text-muted-foreground">{t('apiDocs.apiKeysIntro')}</p>
            <div className="space-y-2">
              <p className="font-medium">{t('apiDocs.apiKeysList')}</p>
              <CodeBlock code={listKeysExample} language={t('apiDocs.bashLabel')} />
            </div>
            <div className="space-y-2">
              <p className="font-medium">{t('apiDocs.apiKeysCreate')}</p>
              <p className="text-xs text-muted-foreground">{t('apiDocs.apiKeysCreateDesc')}</p>
              <CodeBlock code={createKeyExample} language={t('apiDocs.bashLabel')} />
            </div>
            <div className="space-y-2">
              <p className="font-medium">{t('apiDocs.apiKeysDelete')}</p>
              <p className="text-xs text-muted-foreground">{t('apiDocs.apiKeysDeleteDesc')}</p>
              <CodeBlock code={deleteKeyExample} language={t('apiDocs.bashLabel')} />
            </div>
          </CardContent>
        </Card>
      </section>
    </div>
  )
}
