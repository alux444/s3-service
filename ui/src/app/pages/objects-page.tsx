import { type ChangeEvent, type DragEvent, type ReactElement, useState } from 'react'
import type {
  ObjectDeleteInput,
  ObjectUploadInput,
  PresignDownloadInput,
  PresignUploadInput,
} from '../../core/api/contracts'
import { useApiRequestExecutor } from '../hooks/use-api-request-executor'
import { JsonPanel, PageFrame } from './page-frame'

const uploadDefault: ObjectUploadInput = {
  bucket_name: '',
  object_key: '',
  content_type: 'text/plain',
  content_b64: 'aGVsbG8=',
  metadata: {},
}

const deleteDefault: ObjectDeleteInput = {
  bucket_name: '',
  object_key: '',
}

const presignUploadDefault: PresignUploadInput = {
  bucket_name: '',
  object_key: '',
  content_type: 'text/plain',
  expires_in_seconds: 60,
}

const presignDownloadDefault: PresignDownloadInput = {
  bucket_name: '',
  object_key: '',
  expires_in_seconds: 120,
}

type UploadStringField = 'bucket_name' | 'object_key' | 'content_type' | 'content_b64'
type DeleteStringField = 'bucket_name' | 'object_key'
type PresignUploadStringField = 'bucket_name' | 'object_key' | 'content_type'
type PresignDownloadStringField = 'bucket_name' | 'object_key'

const parseMetadata = (rawValue: string): Record<string, string> => {
  return rawValue
    .split(',')
    .map((pair: string): string[] => pair.split(':').map((part: string): string => part.trim()))
    .filter((parts: string[]): boolean => parts.length === 2)
    .reduce((metadata: Record<string, string>, [key, value]: string[]): Record<string, string> => {
      if (key === undefined || value === undefined || key === '') {
        return metadata
      }

      return {
        ...metadata,
        [key]: value,
      }
    }, {})
}

const arrayBufferToBase64 = (buffer: ArrayBuffer): string => {
  const bytes = new Uint8Array(buffer)
  let binary = ''

  bytes.forEach((byte: number): void => {
    binary += String.fromCharCode(byte)
  })

  return btoa(binary)
}

const readFileAsBase64 = async (file: File): Promise<string> => {
  const buffer: ArrayBuffer = await file.arrayBuffer()
  return arrayBufferToBase64(buffer)
}

const resolveContentType = (file: File): string => {
  if (file.type !== '') {
    return file.type
  }

  return 'application/octet-stream'
}

export const ObjectsPage = (): ReactElement => {
  const { executeApiRequest } = useApiRequestExecutor()
  const [uploadDraft, setUploadDraft] = useState<ObjectUploadInput>(uploadDefault)
  const [deleteDraft, setDeleteDraft] = useState<ObjectDeleteInput>(deleteDefault)
  const [presignUploadDraft, setPresignUploadDraft] = useState<PresignUploadInput>(presignUploadDefault)
  const [presignDownloadDraft, setPresignDownloadDraft] = useState<PresignDownloadInput>(presignDownloadDefault)
  const [metadataText, setMetadataText] = useState<string>('source:ui')
  const [selectedUploadFileName, setSelectedUploadFileName] = useState<string>('No file selected.')
  const [uploadSelectionNotice, setUploadSelectionNotice] = useState<string>('Drag a file here or choose a file.')
  const [isDragOverUploadZone, setIsDragOverUploadZone] = useState<boolean>(false)
  const [lastPayload, setLastPayload] = useState<unknown>({ info: 'No object request executed.' })

  const updateUploadField = (field: UploadStringField) => {
    return (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>): void => {
      setUploadDraft({
        ...uploadDraft,
        [field]: event.target.value,
      })
    }
  }

  const updateDeleteField = (field: DeleteStringField) => {
    return (event: ChangeEvent<HTMLInputElement>): void => {
      setDeleteDraft({
        ...deleteDraft,
        [field]: event.target.value,
      })
    }
  }

  const updatePresignUploadField = (field: PresignUploadStringField) => {
    return (event: ChangeEvent<HTMLInputElement>): void => {
      setPresignUploadDraft({
        ...presignUploadDraft,
        [field]: event.target.value,
      })
    }
  }

  const updatePresignDownloadField = (field: PresignDownloadStringField) => {
    return (event: ChangeEvent<HTMLInputElement>): void => {
      setPresignDownloadDraft({
        ...presignDownloadDraft,
        [field]: event.target.value,
      })
    }
  }

  const applySelectedUploadFile = async (file: File): Promise<void> => {
    try {
      const nextContentB64: string = await readFileAsBase64(file)
      const nextContentType: string = resolveContentType(file)

      setUploadDraft((currentDraft: ObjectUploadInput): ObjectUploadInput => {
        const shouldUseFileNameAsKey: boolean = currentDraft.object_key.trim().length === 0

        return {
          ...currentDraft,
          object_key: shouldUseFileNameAsKey ? file.name : currentDraft.object_key,
          content_type: nextContentType,
          content_b64: nextContentB64,
        }
      })
      setSelectedUploadFileName(file.name)
      setUploadSelectionNotice('File loaded into upload payload.')
    } catch {
      setUploadSelectionNotice('Unable to read file. Try a smaller file or another format.')
    }
  }

  const onUploadFileInputChange = (event: ChangeEvent<HTMLInputElement>): void => {
    const selectedFile: File | undefined = event.target.files?.[0]

    if (selectedFile === undefined) {
      return
    }

    void applySelectedUploadFile(selectedFile)
    event.target.value = ''
  }

  const onUploadDrop = (event: DragEvent<HTMLDivElement>): void => {
    event.preventDefault()
    setIsDragOverUploadZone(false)

    const droppedFile: File | undefined = event.dataTransfer.files?.[0]

    if (droppedFile === undefined) {
      return
    }

    void applySelectedUploadFile(droppedFile)
  }

  const onUploadDragOver = (event: DragEvent<HTMLDivElement>): void => {
    event.preventDefault()

    if (!isDragOverUploadZone) {
      setIsDragOverUploadZone(true)
    }
  }

  const onUploadDragLeave = (): void => {
    setIsDragOverUploadZone(false)
  }

  const runUpload = (): void => {
    const requestBody: ObjectUploadInput = {
      ...uploadDraft,
      metadata: parseMetadata(metadataText),
    }

    void executeApiRequest({
      operationName: 'upload_object',
      method: 'POST',
      path: '/v1/objects/upload',
      successStatus: 201,
      requestBody,
      execute: (api) => api.uploadObject(requestBody),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  const runDelete = (): void => {
    void executeApiRequest({
      operationName: 'delete_object',
      method: 'DELETE',
      path: '/v1/objects',
      successStatus: 200,
      requestBody: deleteDraft,
      execute: (api) => api.deleteObject(deleteDraft),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  const runPresignUpload = (): void => {
    void executeApiRequest({
      operationName: 'presign_upload',
      method: 'POST',
      path: '/v1/objects/presign-upload',
      successStatus: 200,
      requestBody: presignUploadDraft,
      execute: (api) => api.presignUpload(presignUploadDraft),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  const runPresignDownload = (): void => {
    void executeApiRequest({
      operationName: 'presign_download',
      method: 'POST',
      path: '/v1/objects/presign-download',
      successStatus: 200,
      requestBody: presignDownloadDraft,
      execute: (api) => api.presignDownload(presignDownloadDraft),
    }).then((payload: unknown): void => {
      if (payload !== undefined) {
        setLastPayload(payload)
      }
    })
  }

  return (
    <PageFrame title="Objects" description="Run upload, delete, and presign object operations.">
      <article className="workbench-card">
        <h2>Upload / Delete / Presign</h2>
        <label className="workbench-field"><span>Upload Bucket</span><input value={uploadDraft.bucket_name} onChange={updateUploadField('bucket_name')} /></label>
        <label className="workbench-field"><span>Upload Object Key</span><input value={uploadDraft.object_key} onChange={updateUploadField('object_key')} /></label>
        <label className="workbench-field"><span>Content Type</span><input value={uploadDraft.content_type} onChange={updateUploadField('content_type')} /></label>
        <div
          className="workbench-field"
          onDrop={onUploadDrop}
          onDragOver={onUploadDragOver}
          onDragLeave={onUploadDragLeave}
        >
          <span>{isDragOverUploadZone ? 'Drop file to load upload content' : 'Drag file here or choose file'}</span>
          <input type="file" onChange={onUploadFileInputChange} />
          <span>{selectedUploadFileName}</span>
          <span>{uploadSelectionNotice}</span>
        </div>
        <label className="workbench-field"><span>Content (Base64)</span><textarea value={uploadDraft.content_b64} rows={4} onChange={updateUploadField('content_b64')} /></label>
        <label className="workbench-field"><span>Metadata (key:value,key:value)</span><input value={metadataText} onChange={(event): void => setMetadataText(event.target.value)} /></label>
        <label className="workbench-field"><span>Delete Bucket</span><input value={deleteDraft.bucket_name} onChange={updateDeleteField('bucket_name')} /></label>
        <label className="workbench-field"><span>Delete Object Key</span><input value={deleteDraft.object_key} onChange={updateDeleteField('object_key')} /></label>
        <label className="workbench-field"><span>Presign Upload Bucket</span><input value={presignUploadDraft.bucket_name} onChange={updatePresignUploadField('bucket_name')} /></label>
        <label className="workbench-field"><span>Presign Upload Object Key</span><input value={presignUploadDraft.object_key} onChange={updatePresignUploadField('object_key')} /></label>
        <label className="workbench-field"><span>Presign Download Bucket</span><input value={presignDownloadDraft.bucket_name} onChange={updatePresignDownloadField('bucket_name')} /></label>
        <label className="workbench-field"><span>Presign Download Object Key</span><input value={presignDownloadDraft.object_key} onChange={updatePresignDownloadField('object_key')} /></label>
        <div className="workbench-actions">
          <button type="button" onClick={runUpload}>POST /v1/objects/upload</button>
          <button type="button" onClick={runDelete}>DELETE /v1/objects</button>
          <button type="button" onClick={runPresignUpload}>POST /v1/objects/presign-upload</button>
          <button type="button" onClick={runPresignDownload}>POST /v1/objects/presign-download</button>
        </div>
      </article>
      <JsonPanel title="Last Response" payload={lastPayload} />
    </PageFrame>
  )
}
