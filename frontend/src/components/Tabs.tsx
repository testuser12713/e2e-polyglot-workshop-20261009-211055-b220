import { NavLink } from 'react-router-dom'

export interface TabItem {
  to: string
  label: string
  end?: boolean
}

export interface TabsProps {
  items: TabItem[]
  ariaLabel?: string
}

function tabClass({ isActive }: { isActive: boolean }): string {
  return isActive ? 'tabs__link tabs__link--active' : 'tabs__link'
}

export default function Tabs({ items, ariaLabel = 'Bereichsnavigation' }: TabsProps) {
  return (
    <nav className="tabs" aria-label={ariaLabel}>
      {items.map((item) => (
        <NavLink key={item.to} to={item.to} end={item.end} className={tabClass}>
          {item.label}
        </NavLink>
      ))}
    </nav>
  )
}
