interface Props {
  children: React.ReactNode
  size?: 'md' | 'lg'
  height?: string
}

export function Modal({ children, size = 'md', height = 'h-[90dvh]' }: Props) {
  const maxW = size === 'lg' ? 'max-w-2xl' : 'max-w-lg'
  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div className={`bg-white dark:bg-gray-800 rounded-xl shadow-xl w-full ${maxW} mx-4 ${height} flex flex-col overflow-hidden`}>
        {children}
      </div>
    </div>
  )
}
