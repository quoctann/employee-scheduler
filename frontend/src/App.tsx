import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { CapacityCheckPanel } from '@/components/capacity/CapacityCheckPanel'
import { CandidatesPanel } from '@/components/candidates/CandidatesPanel'
import { EmployeesPanel } from '@/components/employees/EmployeesPanel'
import { WeeklyRegistrationView } from '@/components/registration/WeeklyRegistrationView'
import { ScheduleView } from '@/components/schedule/ScheduleView'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Toaster } from '@/components/ui/sonner'

const queryClient = new QueryClient()

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <div className="mx-auto max-w-6xl space-y-6 p-6">
        <header>
          <h1 className="text-2xl font-semibold">Employee Scheduler — Demo</h1>
          <p className="text-sm text-muted-foreground">
            Demo only
          </p>
        </header>

        <Tabs defaultValue="schedule">
          <TabsList className="max-w-full overflow-x-auto">
            <TabsTrigger value="schedule">Lịch xếp ca</TabsTrigger>
            <TabsTrigger value="registration">Đăng ký lịch</TabsTrigger>
            <TabsTrigger value="capacity">Capacity Check</TabsTrigger>
            <TabsTrigger value="candidates">Đề xuất thay ca</TabsTrigger>
            <TabsTrigger value="employees">Quản lý nhân viên</TabsTrigger>
          </TabsList>
          {/* forceMount: keep every panel's local component state (mutation
              results in particular) alive across tab switches instead of
              unmounting/discarding it, which is Radix's default. */}
          <TabsContent value="schedule" forceMount>
            <ScheduleView />
          </TabsContent>
          <TabsContent value="registration" forceMount>
            <WeeklyRegistrationView />
          </TabsContent>
          <TabsContent value="capacity" forceMount>
            <CapacityCheckPanel />
          </TabsContent>
          <TabsContent value="candidates" forceMount>
            <CandidatesPanel />
          </TabsContent>
          <TabsContent value="employees" forceMount>
            <EmployeesPanel />
          </TabsContent>
        </Tabs>
      </div>
      <Toaster />
    </QueryClientProvider>
  )
}

export default App
