import { createBrowserRouter } from 'react-router'
import { RequireUser } from './components/RequireUser'
import { ChooseRolePage } from './pages/ChooseRolePage'
import { HiringCardPage } from './pages/HiringCardPage'
import { HomePage } from './pages/HomePage'
import { MatchesPage } from './pages/MatchesPage'
import { MatchPage } from './pages/MatchPage'
import { NewHiringCardPage } from './pages/NewHiringCardPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { ProfilePage } from './pages/ProfilePage'
import { SignInPage } from './pages/SignInPage'

export const router = createBrowserRouter([
  { path: '/sign-in', element: <SignInPage /> },
  {
    element: <RequireUser needsRole={false} />,
    children: [{ path: '/choose-role', element: <ChooseRolePage /> }],
  },
  {
    element: <RequireUser needsRole />,
    children: [
      { path: '/', element: <HomePage /> },
      { path: '/profile', element: <ProfilePage /> },
      { path: '/hiring/new', element: <NewHiringCardPage /> },
      { path: '/hiring/:id', element: <HiringCardPage /> },
      { path: '/matches', element: <MatchesPage /> },
      { path: '/matches/:id', element: <MatchPage /> },
      // The timelines carry the swipe animation library, so they load only
      // when opened.
      {
        path: '/hiring/:id/timeline',
        lazy: () => import('./pages/HiringTimelinePage').then((m) => ({ Component: m.HiringTimelinePage })),
      },
      { path: '/timeline', lazy: () => import('./pages/TimelinePage').then((m) => ({ Component: m.TimelinePage })) },
    ],
  },
  { path: '*', element: <NotFoundPage /> },
])
