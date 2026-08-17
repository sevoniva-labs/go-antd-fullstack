import { App, Upload, type UploadFile, type UploadProps } from 'antd'

export interface AppUploadProps extends UploadProps {
  maxSizeMiB?: number
  allowedMimeTypes?: string[]
}

export function AppUpload({ maxSizeMiB = 20, allowedMimeTypes = [], beforeUpload, ...props }: AppUploadProps) {
  const { message } = App.useApp()
  const guard: UploadProps['beforeUpload'] = async (file, fileList) => {
    if (file.size > maxSizeMiB * 1024 * 1024) {
      message.error(`文件大小不能超过 ${maxSizeMiB} MiB`)
      return Upload.LIST_IGNORE
    }
    if (allowedMimeTypes.length > 0 && file.type && !allowedMimeTypes.includes(file.type)) {
      message.error('文件类型不在允许范围内')
      return Upload.LIST_IGNORE
    }
    if (!beforeUpload) return true
    return beforeUpload(file, fileList)
  }
  return <Upload {...props} beforeUpload={guard} />
}

export function uploadedFileID(file?: UploadFile) {
  const response = file?.response as { id?: string; data?: { id?: string } } | undefined
  return response?.data?.id || response?.id || ''
}
