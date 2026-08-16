{{- define "forge.name" -}}forge{{- end -}}
{{- define "forge.fullname" -}}{{ .Release.Name }}-forge{{- end -}}
{{- define "forge.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{ include "forge.fullname" . }}
{{- else -}}
{{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}
{{- define "forge.secretName" -}}
{{- if .Values.secretEnv.existingSecret -}}
{{ .Values.secretEnv.existingSecret }}
{{- else -}}
{{ include "forge.fullname" . }}
{{- end -}}
{{- end -}}
{{- define "forge.selectorLabels" -}}
app.kubernetes.io/name: {{ include "forge.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
