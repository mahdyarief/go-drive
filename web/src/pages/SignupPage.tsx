import { useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { RegisterSettings } from '@/lib/types'
import { Eye, EyeOff, GalleryVerticalEnd } from 'lucide-react'
import { useAuthStore } from '@/store/auth'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'

export default function SignupPage() {
  const user = useAuthStore((s) => s.user)
  const signUp = useAuthStore((s) => s.signUp)
  const navigate = useNavigate()
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const { data: registerSettings, isLoading: settingsLoading } = useQuery({
    queryKey: ['settings', 'register'],
    queryFn: () => api<RegisterSettings>('/api/settings/register'),
  })
  const signupDisabled = registerSettings?.register_disabled === true

  if (user) {
    return <Navigate to="/orgs" replace />
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')

    if (password !== confirmPassword) {
      setError(t('auth.passwordsDoNotMatch'))
      return
    }
    if (password.length < 8) {
      setError(t('auth.passwordMinLength'))
      return
    }

    setLoading(true)
    try {
      await signUp(name, email, password)
      navigate('/orgs')
      return
    } catch (err) {
      setError(err instanceof Error ? err.message : t('auth.signUpFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="bg-muted flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
      <div className="flex w-full max-w-sm flex-col gap-6">
        <a href="/" className="flex items-center gap-2 self-center font-medium">
          <div className="bg-primary text-primary-foreground flex size-6 items-center justify-center rounded-md">
            <GalleryVerticalEnd className="size-4" />
          </div>
          {t('app.title')}
        </a>
        <Card>
          {settingsLoading ? (
            <CardContent className="py-10" />
          ) : signupDisabled ? (
            <CardContent className="py-10 text-center">
              <p className="text-sm font-medium">{t('auth.signupDisabled')}</p>
              <p className="mt-1 text-xs text-muted-foreground">{t('auth.signupDisabledHint')}</p>
            </CardContent>
          ) : (
            <>
              <CardHeader className="text-center">
                <CardTitle className="text-xl">{t('auth.signUp')}</CardTitle>
                <CardDescription>{t('auth.signUpSubtitle')}</CardDescription>
              </CardHeader>
              <CardContent>
                <form onSubmit={handleSubmit}>
              <div className="grid gap-6">
                {error && (
                  <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3">
                    <p className="text-sm text-destructive">{error}</p>
                  </div>
                )}
                <div className="grid gap-6">
                  <div className="grid gap-2">
                    <Label htmlFor="name">{t('form.name')}</Label>
                    <Input
                      id="name"
                      type="text"
                      placeholder={t('form.namePlaceholder')}
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      required
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="email">{t('form.email')}</Label>
                    <Input
                      id="email"
                      type="email"
                      placeholder={t('form.emailPlaceholder')}
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      required
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="password">{t('form.password')}</Label>
                    <InputGroup>
                      <InputGroupInput
                        id="password"
                        type={showPassword ? 'text' : 'password'}
                        placeholder={t('form.passwordMinPlaceholder')}
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        required
                        minLength={8}
                      />
                      <InputGroupButton
                        size="icon-sm"
                        onClick={() => setShowPassword((v) => !v)}
                        aria-label={showPassword ? t('form.hidePassword') : t('form.showPassword')}
                        title={showPassword ? t('form.hidePassword') : t('form.showPassword')}
                      >
                        {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                      </InputGroupButton>
                    </InputGroup>
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="confirmPassword">
                      {t('form.confirmPassword')}
                    </Label>
                    <InputGroup>
                      <InputGroupInput
                        id="confirmPassword"
                        type={showConfirmPassword ? 'text' : 'password'}
                        placeholder={t('form.confirmPasswordPlaceholder')}
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        required
                        minLength={8}
                      />
                      <InputGroupButton
                        size="icon-sm"
                        onClick={() => setShowConfirmPassword((v) => !v)}
                        aria-label={showConfirmPassword ? t('form.hidePassword') : t('form.showPassword')}
                        title={showConfirmPassword ? t('form.hidePassword') : t('form.showPassword')}
                      >
                        {showConfirmPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                      </InputGroupButton>
                    </InputGroup>
                  </div>
                  <Button type="submit" className="w-full" disabled={loading}>
                    {loading ? t('auth.creatingAccount') : t('auth.signUp')}
                  </Button>
                </div>
                <div className="text-center text-sm">
                  {t('auth.hasAccount')}{' '}
                  <Link
                    to="/login"
                    className="underline underline-offset-4"
                  >
                    {t('auth.signIn')}
                  </Link>
                </div>
              </div>
            </form>
          </CardContent>
            </>
          )}
        </Card>
        <div className="text-muted-foreground text-balance text-center text-xs [&_a]:underline [&_a]:underline-offset-4 [&_a]:hover:text-primary">
          {t('auth.termsNotice')}{' '}
          <a href="#">{t('auth.termsOfService')}</a>{' '}
          {t('auth.and')}{' '}
          <a href="#">{t('auth.privacyPolicy')}</a>.
        </div>
      </div>
    </div>
  )
}
