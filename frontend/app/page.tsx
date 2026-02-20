import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { 
  Truck, 
  MapPin, 
  WifiOff, 
  Smartphone, 
  ShieldCheck, 
  Clock, 
  CheckCircle2,
  ArrowRight,
  Phone,
  Mail,
  ChevronRight,
} from 'lucide-react'
import { ThemeToggle } from '@/components/theme-toggle'

const features = [
  {
    icon: WifiOff,
    title: 'Works Offline',
    description: 'Full functionality without internet. GPS, signatures, and photos captured locally, synced automatically when connected.',
  },
  {
    icon: MapPin,
    title: 'Real-Time Tracking',
    description: 'See driver locations on a map. Know where every truck is without calling.',
  },
  {
    icon: ShieldCheck,
    title: 'Proof of Delivery',
    description: 'Digital signatures, photos, and timestamps. Resolve disputes with verifiable evidence.',
  },
  {
    icon: Smartphone,
    title: 'Simple Driver App',
    description: 'Android app designed for drivers. Works on budget smartphones. Learns in minutes.',
  },
  {
    icon: Clock,
    title: 'Save Hours Daily',
    description: 'Reduce driver phone calls by 40-60%. Dispatchers focus on exceptions, not status checks.',
  },
]

const howItWorks = [
  {
    step: '01',
    title: 'Create',
    description: 'Add shipments and assign to drivers.',
  },
  {
    step: '02',
    title: 'Track',
    description: 'Monitor locations from the dashboard.',
  },
  {
    step: '03',
    title: 'Deliver',
    description: 'Driver captures signature and photos.',
  },
  {
    step: '04',
    title: 'Sync',
    description: 'Data syncs when connected.',
  },
]

const pricingPlans = [
  {
    name: 'Starter',
    price: '$15',
    period: '/driver/mo',
    description: 'For small fleets',
    features: [
      'Up to 10 drivers',
      'Unlimited shipments',
      'GPS tracking',
      'Proof of delivery',
      'Email support',
    ],
  },
  {
    name: 'Professional',
    price: '$20',
    period: '/driver/mo',
    description: 'For growing companies',
    features: [
      'Unlimited drivers',
      'Photo proof of delivery',
      'SMS notifications',
      'Analytics dashboard',
      'Priority support',
      'API access',
    ],
    featured: true,
  },
  {
    name: 'Enterprise',
    price: 'Custom',
    period: '',
    description: 'For large fleets',
    features: [
      'Custom integrations',
      'Dedicated manager',
      'On-site training',
      'SLA guarantee',
      'White-label option',
    ],
  },
]

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-white text-black antialiased dark:bg-black dark:text-white">
      <nav className="fixed top-0 left-0 right-0 z-50 border-b border-black/5 bg-white/80 backdrop-blur-xl dark:border-white/5 dark:bg-black/80">
        <div className="mx-auto flex h-12 max-w-6xl items-center justify-between px-6">
          <div className="flex items-center gap-2">
            <Truck className="h-5 w-5" />
            <span className="text-sm font-semibold tracking-tight">EthioTrack</span>
          </div>
          <div className="flex items-center gap-1">
            <Link href="/track" className="hidden sm:block">
              <Button variant="ghost" size="sm" className="text-xs">
                Track
              </Button>
            </Link>
            <ThemeToggle />
            <Link href="/login">
              <Button variant="ghost" size="sm" className="text-xs">
                Sign In
              </Button>
            </Link>
            <Link href="/login">
              <Button size="sm" className="text-xs">
                Get Started
              </Button>
            </Link>
          </div>
        </div>
      </nav>

      <main className="pt-12">
        <section className="relative overflow-hidden">
          <div className="absolute inset-0 bg-gradient-to-b from-black/[0.02] to-transparent dark:from-white/[0.02]" />
          <div className="mx-auto max-w-5xl px-6 pt-32 pb-20 text-center sm:pt-40 sm:pb-32">
            <p className="text-xs font-medium uppercase tracking-widest text-black/60 dark:text-white/60">
              Built for Ethiopian Infrastructure
            </p>
            <h1 className="mt-6 text-4xl font-semibold tracking-tight sm:text-5xl md:text-6xl lg:text-7xl">
              Ethiopian Logistics
              <br />
              <span className="text-black/40 dark:text-white/40">Tracking That Works.</span>
            </h1>
            <p className="mx-auto mt-6 max-w-2xl text-lg text-black/60 dark:text-white/60 sm:text-xl">
              Track your fleet across Ethiopia—offline or online. 
              No more blind spots. No more delivery disputes.
            </p>
            <div className="mt-10 flex flex-col items-center justify-center gap-3 sm:flex-row sm:gap-4">
              <Link href="/login">
                <Button size="lg" className="rounded-full px-8">
                  Start Free Trial
                  <ArrowRight className="ml-2 h-4 w-4" />
                </Button>
              </Link>
              <Link href="/track">
                <Button size="lg" variant="outline" className="rounded-full px-8">
                  Track a Shipment
                </Button>
              </Link>
            </div>
          </div>
        </section>

        <section className="border-y border-black/5 bg-black/[0.02] dark:border-white/5 dark:bg-white/[0.02]">
          <div className="mx-auto max-w-5xl px-6 py-16">
            <div className="grid grid-cols-2 gap-8 sm:grid-cols-4">
              {[
                { value: '15,000+', label: 'Trucks in Ethiopia' },
                { value: '40%', label: 'Time Saved' },
                { value: '99%', label: 'Sync Success' },
                { value: '24/7', label: 'Offline Ready' },
              ].map((stat) => (
                <div key={stat.label} className="text-center">
                  <div className="text-2xl font-semibold tracking-tight sm:text-3xl">
                    {stat.value}
                  </div>
                  <div className="mt-1 text-xs text-black/60 dark:text-white/60">
                    {stat.label}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section id="features" className="mx-auto max-w-5xl px-6 py-24 sm:py-32">
          <div className="text-center">
            <h2 className="text-3xl font-semibold tracking-tight sm:text-4xl">
              Fleet Tracking.
              <br />
              <span className="text-black/40 dark:text-white/40">Built for Ethiopia.</span>
            </h2>
            <p className="mx-auto mt-4 max-w-xl text-black/60 dark:text-white/60">
              Global platforms require constant internet. We built for Ethiopia&apos;s reality.
            </p>
          </div>
          <div className="mt-16 grid gap-12 sm:grid-cols-2 lg:grid-cols-3">
            {features.map((feature) => (
              <div key={feature.title} className="group">
                <div className="mb-4 inline-flex rounded-full bg-black/5 p-3 dark:bg-white/5">
                  <feature.icon className="h-5 w-5" />
                </div>
                <h3 className="text-base font-semibold">
                  {feature.title}
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-black/60 dark:text-white/60">
                  {feature.description}
                </p>
              </div>
            ))}
          </div>
        </section>

        <section className="border-y border-black/5 bg-black text-white dark:border-white/5 dark:bg-white dark:text-black">
          <div className="mx-auto max-w-5xl px-6 py-24 sm:py-32">
            <div className="grid items-center gap-16 lg:grid-cols-2">
              <div>
                <h2 className="text-3xl font-semibold tracking-tight sm:text-4xl">
                  Works Offline.
                  <br />
                  Syncs Automatically.
                </h2>
                <p className="mt-4 text-lg text-white/60 dark:text-black/60">
                  Our driver app stores everything locally. When connectivity returns, 
                  it all syncs automatically. No data loss.
                </p>
                <ul className="mt-8 space-y-4">
                  {[
                    'GPS tracking without internet',
                    'Signature capture offline',
                    'Photo proof stored locally',
                    'Auto-sync when connected',
                  ].map((item) => (
                    <li key={item} className="flex items-center gap-3">
                      <div className="flex h-5 w-5 items-center justify-center rounded-full bg-white dark:bg-black">
                        <CheckCircle2 className="h-3 w-3 text-black dark:text-white" />
                      </div>
                      <span className="text-sm text-white/80 dark:text-black/80">{item}</span>
                    </li>
                  ))}
                </ul>
                <div className="mt-10">
                  <Link href="/login">
                    <Button variant="outline" className="rounded-full border-white/20 text-white hover:bg-white/10 dark:border-black/20 dark:text-black dark:hover:bg-black/5">
                      Learn more
                      <ChevronRight className="ml-1 h-4 w-4" />
                    </Button>
                  </Link>
                </div>
              </div>
              <div className="relative mx-auto max-w-xs">
                <div className="aspect-[9/16] rounded-[2.5rem] border border-white/10 bg-white/5 p-2 dark:border-black/10 dark:bg-black/5">
                  <div className="flex h-full flex-col items-center justify-center rounded-[2rem] border border-white/5 bg-white/[0.02] dark:border-black/5 dark:bg-black/[0.02]">
                    <div className="rounded-full border border-white/10 p-4 dark:border-black/10">
                      <Smartphone className="h-8 w-8 text-white/40 dark:text-black/40" />
                    </div>
                    <p className="mt-4 text-sm font-medium text-white/60 dark:text-black/60">Driver App</p>
                    <p className="mt-1 text-xs text-white/40 dark:text-black/40">Android</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section id="how-it-works" className="mx-auto max-w-5xl px-6 py-24 sm:py-32">
          <div className="text-center">
            <h2 className="text-3xl font-semibold tracking-tight sm:text-4xl">
              Simple.
              <br />
              <span className="text-black/40 dark:text-white/40">As it should be.</span>
            </h2>
          </div>
          <div className="mt-16 grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
            {howItWorks.map((item, index) => (
              <div key={item.step} className="text-center">
                <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full border border-black/10 dark:border-white/10">
                  <span className="text-sm font-semibold text-black/40 dark:text-white/40">{item.step}</span>
                </div>
                <h3 className="mt-6 text-base font-semibold">
                  {item.title}
                </h3>
                <p className="mt-2 text-sm text-black/60 dark:text-white/60">
                  {item.description}
                </p>
              </div>
            ))}
          </div>
        </section>

        <section id="pricing" className="border-t border-black/5 bg-black/[0.02] dark:border-white/5 dark:bg-white/[0.02]">
          <div className="mx-auto max-w-5xl px-6 py-24 sm:py-32">
            <div className="text-center">
              <h2 className="text-3xl font-semibold tracking-tight sm:text-4xl">
                Pricing.
              </h2>
              <p className="mx-auto mt-4 max-w-xl text-black/60 dark:text-white/60">
                Simple pricing. No hidden fees.
              </p>
            </div>
            <div className="mt-16 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
              {pricingPlans.map((plan) => (
                <div 
                  key={plan.name} 
                  className={`rounded-2xl p-8 ${
                    plan.featured 
                      ? 'bg-black text-white dark:bg-white dark:text-black' 
                      : 'border border-black/10 dark:border-white/10'
                  }`}
                >
                  <h3 className="text-sm font-medium uppercase tracking-wide opacity-60">
                    {plan.name}
                  </h3>
                  <div className="mt-4 flex items-baseline">
                    <span className="text-4xl font-semibold tracking-tight">
                      {plan.price}
                    </span>
                    <span className="ml-1 text-sm opacity-60">{plan.period}</span>
                  </div>
                  <p className="mt-2 text-sm opacity-60">
                    {plan.description}
                  </p>
                  <ul className="mt-8 space-y-3">
                    {plan.features.map((feature) => (
                      <li key={feature} className="flex items-center gap-3 text-sm">
                        <CheckCircle2 className="h-4 w-4 flex-shrink-0 opacity-60" />
                        <span className="opacity-80">{feature}</span>
                      </li>
                    ))}
                  </ul>
                  <Link href="/login" className="mt-8 block">
                    <Button 
                      className="w-full rounded-full"
                      variant={plan.featured ? 'outline' : 'default'}
                    >
                      Get Started
                    </Button>
                  </Link>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="mx-auto max-w-5xl px-6 py-24 text-center sm:py-32">
          <h2 className="text-3xl font-semibold tracking-tight sm:text-4xl">
            Ready to transform
            <br />
            your fleet?
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-black/60 dark:text-white/60">
            Join trucking companies across Ethiopia saving time and ending delivery disputes.
          </p>
          <div className="mt-10 flex flex-col items-center justify-center gap-3 sm:flex-row sm:gap-4">
            <Link href="/login">
              <Button size="lg" className="rounded-full px-8">
                Start Free Trial
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </Link>
            <Link href="/track">
              <Button size="lg" variant="outline" className="rounded-full px-8">
                Track a Shipment
              </Button>
            </Link>
          </div>
        </section>
      </main>

      <footer className="border-t border-black/5 bg-black/[0.02] dark:border-white/5 dark:bg-white/[0.02]">
        <div className="mx-auto max-w-5xl px-6 py-12">
          <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <div className="flex items-center gap-2">
                <Truck className="h-5 w-5" />
                <span className="text-sm font-semibold">EthioTrack</span>
              </div>
              <p className="mt-4 text-xs leading-relaxed text-black/60 dark:text-white/60">
                Offline-first logistics tracking built for Ethiopian trucking.
              </p>
            </div>
            <div>
              <h4 className="text-xs font-medium uppercase tracking-wide text-black/40 dark:text-white/40">Product</h4>
              <ul className="mt-4 space-y-2 text-sm text-black/60 dark:text-white/60">
                <li><Link href="#features" className="hover:text-black dark:hover:text-white">Features</Link></li>
                <li><Link href="#pricing" className="hover:text-black dark:hover:text-white">Pricing</Link></li>
                <li><Link href="/track" className="hover:text-black dark:hover:text-white">Track Shipment</Link></li>
              </ul>
            </div>
            <div>
              <h4 className="text-xs font-medium uppercase tracking-wide text-black/40 dark:text-white/40">Company</h4>
              <ul className="mt-4 space-y-2 text-sm text-black/60 dark:text-white/60">
                <li><Link href="#" className="hover:text-black dark:hover:text-white">About</Link></li>
                <li><Link href="#" className="hover:text-black dark:hover:text-white">Blog</Link></li>
                <li><Link href="#" className="hover:text-black dark:hover:text-white">Contact</Link></li>
              </ul>
            </div>
            <div>
              <h4 className="text-xs font-medium uppercase tracking-wide text-black/40 dark:text-white/40">Contact</h4>
              <ul className="mt-4 space-y-3 text-sm text-black/60 dark:text-white/60">
                <li className="flex items-center gap-2">
                  <Phone className="h-3 w-3" />
                  <span>+251 911 123 456</span>
                </li>
                <li className="flex items-center gap-2">
                  <Mail className="h-3 w-3" />
                  <span>info@ethiotrack.et</span>
                </li>
              </ul>
            </div>
          </div>
          <div className="mt-12 border-t border-black/5 pt-8 text-center text-xs text-black/40 dark:border-white/5 dark:text-white/40">
            <p>&copy; {new Date().getFullYear()} EthioTrack. All rights reserved.</p>
          </div>
        </div>
      </footer>
    </div>
  )
}
