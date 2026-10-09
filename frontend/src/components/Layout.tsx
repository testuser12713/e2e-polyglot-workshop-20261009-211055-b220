import { useState } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

function navClass(isActive: boolean): string {
  return isActive ? 'topnav__link topnav__link--active' : 'topnav__link'
}

export default function Layout() {
  const { employee, isAuthenticated, logout } = useAuth()
  const location = useLocation()
  const [menuOpen, setMenuOpen] = useState(false)
  const workshopActive = location.pathname.startsWith('/werkstatt')

  const closeMenu = () => setMenuOpen(false)

  return (
    <div className="app-shell">
      <header className="topnav">
        <div className="container topnav__inner">
          <Link to="/" className="wordmark" onClick={closeMenu}>
            <span className="wordmark__name">Kfz-Werkstatt</span>
          </Link>

          <button
            type="button"
            className="topnav__menu-toggle"
            aria-label={menuOpen ? 'Menü schließen' : 'Menü öffnen'}
            aria-expanded={menuOpen}
            aria-controls="topnav-panel"
            onClick={() => setMenuOpen((open) => !open)}
          >
            <span className="topnav__menu-icon" aria-hidden="true" />
          </button>

          <div
            id="topnav-panel"
            className={menuOpen ? 'topnav__panel topnav__panel--open' : 'topnav__panel'}
          >
            <nav className="topnav__nav" aria-label="Hauptnavigation">
              <NavLink
                to="/"
                end
                className={navClass(!workshopActive)}
                onClick={closeMenu}
              >
                <span className="topnav__label">Kundenbereich</span>
              </NavLink>
              <NavLink
                to={isAuthenticated ? '/werkstatt/auftraege' : '/werkstatt/login'}
                className={navClass(workshopActive)}
                onClick={closeMenu}
              >
                <span className="topnav__label">Werkstattbereich</span>
              </NavLink>
            </nav>

            <div className="topnav__account">
              {isAuthenticated && employee ? (
                <>
                  <span className="topnav__employee">{employee.name}</span>
                  <button type="button" className="btn btn--ghost btn--sm" onClick={logout}>
                    Abmelden
                  </button>
                </>
              ) : (
                <NavLink to="/werkstatt/login" className="topnav__login" onClick={closeMenu}>
                  Werkstatt-Anmeldung
                </NavLink>
              )}
            </div>
          </div>
        </div>
      </header>

      <main className="page">
        <div className="container">
          <Outlet />
        </div>
      </main>

      <footer className="site-footer">
        <div className="container site-footer__inner">
          <p className="site-footer__brand">Kfz-Werkstatt Kundenportal</p>
          <nav className="site-footer__links" aria-label="Rechtliches">
            <Link to="/impressum">Impressum</Link>
            <Link to="/datenschutz">Datenschutzerklärung</Link>
          </nav>
        </div>
      </footer>
    </div>
  )
}
