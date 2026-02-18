import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { FieldSchema } from '@/types'

interface MetadataFormProps {
  schema: FieldSchema[]
  values: Record<string, string>
  onChange: (values: Record<string, string>) => void
  errors?: Record<string, string>
}

/**
 * Validate metadata values against schema.
 * Returns a map of field key → error message (empty if all valid).
 */
export function validateMetadata(
  schema: FieldSchema[],
  values: Record<string, string>,
): Record<string, string> {
  const errors: Record<string, string> = {}

  for (const field of schema) {
    const value = values[field.key] ?? ''

    // Required check
    if (field.required && !value.trim()) {
      errors[field.key] = `${field.label}は必須です`
      continue
    }

    // Skip further validation if empty and not required
    if (!value.trim()) continue

    switch (field.type) {
      case 'integer': {
        const num = Number(value)
        if (!Number.isInteger(num)) {
          errors[field.key] = '整数を入力してください'
          break
        }
        const min = field.constraints?.min as number | undefined
        const max = field.constraints?.max as number | undefined
        if (min !== undefined && num < min) {
          errors[field.key] = `${min}以上の値を入力してください`
        } else if (max !== undefined && num > max) {
          errors[field.key] = `${max}以下の値を入力してください`
        }
        break
      }
      case 'float': {
        const num = Number(value)
        if (isNaN(num)) {
          errors[field.key] = '数値を入力してください'
          break
        }
        const min = field.constraints?.min as number | undefined
        const max = field.constraints?.max as number | undefined
        if (min !== undefined && num < min) {
          errors[field.key] = `${min}以上の値を入力してください`
        } else if (max !== undefined && num > max) {
          errors[field.key] = `${max}以下の値を入力してください`
        }
        break
      }
      case 'url': {
        try {
          new URL(value)
        } catch {
          errors[field.key] = '有効なURLを入力してください'
        }
        break
      }
      case 'date': {
        // YYYY-MM-DD format
        if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || isNaN(Date.parse(value))) {
          errors[field.key] = 'YYYY-MM-DD形式で入力してください'
        }
        break
      }
      case 'enum': {
        const allowed = (field.constraints?.values as string[]) ?? []
        if (allowed.length > 0 && !allowed.includes(value)) {
          errors[field.key] = '選択肢から選んでください'
        }
        break
      }
    }
  }

  return errors
}

export function MetadataForm({ schema, values, onChange, errors = {} }: MetadataFormProps) {
  if (schema.length === 0) return null

  const handleFieldChange = (key: string, value: string) => {
    onChange({ ...values, [key]: value })
  }

  return (
    <div className="space-y-4 pt-4 border-t">
      <h4 className="text-sm font-medium text-muted-foreground">タイプ固有情報</h4>
      {schema.map((field) => (
        <div key={field.key} className="space-y-2">
          <Label>
            {field.label}
            {field.required && ' *'}
          </Label>
          {renderField(field, values[field.key] ?? '', (v) => handleFieldChange(field.key, v))}
          {errors[field.key] && (
            <p className="text-xs text-destructive">{errors[field.key]}</p>
          )}
        </div>
      ))}
    </div>
  )
}

function renderField(
  field: FieldSchema,
  value: string,
  onChange: (value: string) => void
) {
  switch (field.type) {
    case 'string':
      return (
        <Input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.label}
        />
      )

    case 'integer':
    case 'float': {
      const min = field.constraints?.min as number | undefined
      const max = field.constraints?.max as number | undefined
      return (
        <Input
          type="number"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          min={min}
          max={max}
          step={field.type === 'float' ? '0.1' : '1'}
          placeholder={field.label}
        />
      )
    }

    case 'boolean':
      return (
        <div className="flex items-center space-x-2">
          <Checkbox
            checked={value === 'true'}
            onCheckedChange={(checked) => onChange(checked ? 'true' : 'false')}
          />
          <span className="text-sm">{field.label}</span>
        </div>
      )

    case 'enum': {
      const enumValues = (field.constraints?.values as string[]) ?? []
      return (
        <Select value={value || '_none'} onValueChange={(v) => onChange(v === '_none' ? '' : v)}>
          <SelectTrigger>
            <SelectValue placeholder={`${field.label}を選択`} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="_none">未選択</SelectItem>
            {enumValues.map((v) => (
              <SelectItem key={v} value={v}>
                {v}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )
    }

    case 'date':
      return (
        <Input
          type="date"
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      )

    case 'url':
      return (
        <Input
          type="url"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="https://..."
        />
      )

    default:
      return (
        <Input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.label}
        />
      )
  }
}
