import { IconExternalLink } from "@tabler/icons-react"
import { useTranslation } from "react-i18next"

import type { ChannelConfig } from "@/api/channels"
import { maskedSecretPlaceholder } from "@/components/secret-placeholder"
import { Field, KeyInput } from "@/components/shared-form"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

interface WebchatFormProps {
  config: ChannelConfig
  onChange: (key: string, value: unknown) => void
  isEdit: boolean
  enabled?: boolean
  fieldErrors?: Record<string, string>
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : ""
}

function asNumber(value: unknown): string {
  if (typeof value === "number") return String(value)
  if (typeof value === "string" && value !== "") return value
  return ""
}

function asStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === "string")
}

function buildOpenChatUrl(config: ChannelConfig): string | null {
  const host = asString(config.host) || "localhost"
  const port = asNumber(config.port)
  if (!port) return null
  const displayHost =
    host === "0.0.0.0" || host === "" ? "localhost" : host
  return `http://${displayHost}:${port}`
}

export function WebchatForm({
  config,
  onChange,
  isEdit,
  enabled,
  fieldErrors = {},
}: WebchatFormProps) {
  const { t } = useTranslation()

  const passwordExtraHint =
    isEdit && asString(config.password)
      ? ` ${t("channels.field.secretHintSet")}`
      : ""

  const openChatUrl = buildOpenChatUrl(config)

  return (
    <div className="space-y-5">
      {/* Open Chat button — shown when enabled and port is set */}
      {enabled && openChatUrl && (
        <div className="border-border/60 bg-violet-500/5 flex items-center justify-between rounded-lg border border-violet-500/20 px-4 py-3">
          <div className="min-w-0">
            <p className="text-sm font-medium">
              {t("channels.webchat.openChatTitle")}
            </p>
            <p className="text-muted-foreground mt-0.5 text-xs">
              {openChatUrl}
            </p>
          </div>
          <Button
            asChild
            size="sm"
            variant="secondary"
            className="ml-4 shrink-0 gap-2"
          >
            <a href={openChatUrl} target="_blank" rel="noreferrer">
              <IconExternalLink className="size-3.5" />
              {t("channels.webchat.openChat")}
            </a>
          </Button>
        </div>
      )}

      {/* Host */}
      <Field
        label={t("channels.field.host")}
        hint={t("channels.form.desc.host")}
        error={fieldErrors.host}
      >
        <Input
          value={asString(config.host)}
          onChange={(e) => onChange("host", e.target.value)}
          placeholder="0.0.0.0"
        />
      </Field>

      {/* Port */}
      <Field
        label={t("channels.field.port")}
        hint={t("channels.form.desc.port")}
        error={fieldErrors.port}
      >
        <Input
          type="number"
          value={asNumber(config.port)}
          onChange={(e) =>
            onChange("port", e.target.value === "" ? 0 : Number(e.target.value))
          }
          placeholder="18888"
        />
      </Field>

      {/* Username */}
      <Field
        label={t("channels.webchat.username")}
        hint={t("channels.webchat.usernameHint")}
        error={fieldErrors.username}
      >
        <Input
          value={asString(config.username)}
          onChange={(e) => onChange("username", e.target.value)}
          placeholder={t("channels.webchat.usernamePlaceholder")}
          autoComplete="off"
        />
      </Field>

      {/* Password */}
      <Field
        label={t("channels.webchat.password")}
        hint={`${t("channels.webchat.passwordHint")}${passwordExtraHint}`}
        error={fieldErrors.password}
      >
        <KeyInput
          value={asString(config._password)}
          onChange={(v) => onChange("_password", v)}
          placeholder={maskedSecretPlaceholder(
            config.password,
            t("channels.webchat.passwordPlaceholder"),
          )}
        />
      </Field>

      {/* Allow From */}
      <Field
        label={t("channels.field.allowFrom")}
        hint={t("channels.form.desc.allowFrom")}
      >
        <Input
          value={asStringArray(config.allow_from).join(", ")}
          onChange={(e) =>
            onChange(
              "allow_from",
              e.target.value
                .split(",")
                .map((s: string) => s.trim())
                .filter(Boolean),
            )
          }
          placeholder={t("channels.field.allowFromPlaceholder")}
        />
      </Field>
    </div>
  )
}
