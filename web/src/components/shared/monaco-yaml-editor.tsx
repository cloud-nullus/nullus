import { useCallback, useRef } from 'react'
import Editor from '@monaco-editor/react'
import type { Monaco } from '@monaco-editor/react'
import { configureMonacoYaml } from 'monaco-yaml'

import { useThemeStore } from '../../stores/theme-store'
import { cn } from '../../lib/utils'

interface MonacoYamlEditorProps {
  value: string
  onChange: (value: string) => void
  height?: string
  readOnly?: boolean
  className?: string
  ariaLabel?: string
}

/**
 * Monaco 기반 YAML 에디터.
 *
 * 설치 마법사와 배포된 스택 설정 편집이 같은 엔진·같은 검증을 쓰도록 한 곳에
 * 모아 둔다. 예전에는 설치 마법사 안에 monaco 설정이 인라인으로 박혀 있어
 * 다른 화면이 YAML 을 편집하려면 그 설정을 복사해야 했다.
 *
 * 같은 폴더의 `yaml-editor` 는 textarea 기반의 가벼운 편집기다. 이름이
 * 헷갈리지 않도록 이쪽은 엔진 이름을 그대로 달았다.
 */
export function MonacoYamlEditor({
  value,
  onChange,
  height = '520px',
  readOnly = false,
  className,
  ariaLabel,
}: MonacoYamlEditorProps) {
  const theme = useThemeStore((state) => state.theme)
  const configuredRef = useRef(false)

  const handleBeforeMount = useCallback((monaco: Monaco) => {
    if (configuredRef.current) return
    configureMonacoYaml(monaco, {
      validate: true,
      completion: false,
      hover: true,
      format: true,
      // 스키마를 네트워크로 받아오지 않는다. 폐쇄망 설치에서 조용히 멈춘다.
      enableSchemaRequest: false,
      schemas: [],
    })
    configuredRef.current = true
  }, [])

  return (
    <div
      className={cn(
        'overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] p-2',
        className,
      )}
      aria-label={ariaLabel}
    >
      <Editor
        beforeMount={handleBeforeMount}
        height={height}
        language="yaml"
        theme={theme === 'dark' ? 'vs-dark' : 'vs-light'}
        value={value}
        onChange={(next) => onChange(next ?? '')}
        options={{
          minimap: { enabled: false },
          fontSize: 13,
          lineNumbers: 'on',
          scrollBeyondLastLine: false,
          wordWrap: 'on',
          tabSize: 2,
          readOnly,
        }}
      />
    </div>
  )
}
