import { createBrowserRouter } from 'react-router'
import { RequireUser } from './components/RequireUser'
import { ChooseRolePage } from './pages/ChooseRolePage'
import { HomePage } from './pages/HomePage'
import { NotFoundPage } from './pages/NotFoundPage'
import { SignInPage } from './pages/SignInPage'

export const router = createBrowserRouter([
  { path: '/sign-in', element: <SignInPage /> },
  {
    element: <RequireUser needsRole={false} />,
    children: [{ path: '/choose-role', element: <ChooseRolePage /> }],
  },
  {
    element: <RequireUser needsRole />,
    children: [{ path: '/', element: <HomePage /> }],
  },
  { path: '*', element: <NotFoundPage /> },
])
