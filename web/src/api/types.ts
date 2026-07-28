export type PreviewKind = 'markdown' | 'text' | 'image' | 'unsupported'
export type FileKind = 'file' | 'directory'

export interface FsItem {
  path: string
  name: string
  kind: FileKind
  previewKind: PreviewKind
  size: number
  modifiedAt: string
  mime: string
}

export interface SessionResponse {
  authenticated: boolean
  username?: string
}

export interface FileListResponse {
  items: FsItem[]
}

export interface BrowseDirEntry {
  name: string
  path: string
}

export interface BrowseResponse {
  path: string
  dirs: BrowseDirEntry[]
}

export interface FileMetaResponse {
  item: FsItem
}

export interface TextResponse {
  path: string
  content: string
  encoding: string
  size: number
  modifiedAt: string
}

export interface SettingsResponse {
  enableTerminal: boolean
}
