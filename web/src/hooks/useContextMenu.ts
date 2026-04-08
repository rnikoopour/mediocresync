import { useState, useEffect, useLayoutEffect, useRef } from 'react'

export interface MenuPosition {
  x: number
  y: number
}

export function useContextMenu() {
  const [menu, setMenu] = useState<MenuPosition | null>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!menu) return
    function close(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenu(null)
    }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [menu])

  useLayoutEffect(() => {
    if (!menu || !menuRef.current) return
    const el = menuRef.current
    const r = el.getBoundingClientRect()
    if (r.right  > window.innerWidth)  el.style.left = `${menu.x - r.width}px`
    if (r.bottom > window.innerHeight) el.style.top  = `${menu.y - r.height}px`
  }, [menu])

  return { menu, setMenu, menuRef }
}
