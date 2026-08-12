import type { ShareLink, TrackedLink, TrackedLinkEvent, UploadLink } from '@/lib/types'

export interface LinksData<T> {
  links: T[]
}

export interface EventsData {
  events: TrackedLinkEvent[]
}

export type LinkKind = 'share' | 'upload' | 'tracked'

export type AnyLink = ShareLink | UploadLink | TrackedLink

export interface LinkForm {
  name: string
  access: string
  fileId: string
  folderId: string
  password: string
  expiresAt: string
  maxDownloads: string
  maxFiles: string
  maxFileSize: string
  requireEmail: boolean
  maxViews: string
  description: string
  isActive: boolean
}

export const emptyForm: LinkForm = {
  name: '',
  access: 'download',
  fileId: '',
  folderId: '',
  password: '',
  expiresAt: '',
  maxDownloads: '',
  maxFiles: '',
  maxFileSize: '',
  requireEmail: false,
  maxViews: '',
  description: '',
  isActive: true,
}

export const resourcePath = (kind: LinkKind) => {
  if (kind === 'share') return 'share-links'
  if (kind === 'upload') return 'upload-links'
  return 'tracked-links'
}
