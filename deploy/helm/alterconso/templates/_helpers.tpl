{{/*
Helpers standards Helm. Convention répandue (Bitnami, prometheus-community,
etc.) : on définit un nom court, un fullname tronqué à 63 chars, et deux
sets de labels (un complet pour metadata.labels, un stable pour selectors).
*/}}

{{/*
Nom court du chart : par défaut .Chart.Name, surchargeable via nameOverride.
Tronqué à 63 chars (limite DNS k8s sur certains champs).
*/}}
{{- define "alterconso.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Nom complet : combine release name + chart name. Évite la collision si
plusieurs releases du même chart cohabitent dans un namespace.
- Si .Values.fullnameOverride est défini → on l'utilise tel quel.
- Sinon, si la release name contient déjà le chart name (ex: helm install
  alterconso ./alterconso), on évite "alterconso-alterconso" et on garde
  juste "alterconso".
- Sinon "<release>-<chart>".
*/}}
{{- define "alterconso.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Chart name + version. Utilisé en label helm.sh/chart pour qu'on retrouve
quelle version du chart a déployé telle ressource.
*/}}
{{- define "alterconso.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Labels appliqués sur metadata.labels de toutes les ressources. Suit la
convention k8s "Recommended Labels" (app.kubernetes.io/*) plus
helm.sh/chart pour la traçabilité.
*/}}
{{- define "alterconso.labels" -}}
helm.sh/chart: {{ include "alterconso.chart" . }}
{{ include "alterconso.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels : sous-ensemble STABLE des labels. À ne JAMAIS changer
au cours de la vie d'une release, sinon le Deployment ne peut plus matcher
ses Pods (les selectors sont immuables après création). C'est pour ça
qu'on garde version et chart hors d'ici.
*/}}
{{- define "alterconso.selectorLabels" -}}
app.kubernetes.io/name: {{ include "alterconso.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Nom du ServiceAccount. Si .Values.serviceAccount.create est true, on dérive
de fullname (sauf si l'utilisateur a fourni un nom custom). Sinon "default".
*/}}
{{- define "alterconso.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "alterconso.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Nom du Secret de l'app — soit existingSecret pointé, soit un nom dérivé
du chart.
*/}}
{{- define "alterconso.secretName" -}}
{{- .Values.secrets.existingSecret | default (include "alterconso.fullname" .) }}
{{- end }}
