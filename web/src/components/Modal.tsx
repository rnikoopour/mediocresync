import { useEffect } from 'react'

interface Props {
  children: React.ReactNode
  size?: 'md' | 'lg'
  height?: string
  onClose: () => void
}

export function Modal({ children, size = 'md', height = 'h-[90dvh]', onClose }: Props) {
  const maxW = size === 'lg' ? 'max-w-2xl' : 'max-w-lg'

  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [onClose])

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div className={`bg-white dark:bg-gray-800 rounded-xl shadow-xl w-full ${maxW} mx-4 ${height} flex flex-col overflow-hidden`}>
        {children}
      </div>
    </div>
  )
}
