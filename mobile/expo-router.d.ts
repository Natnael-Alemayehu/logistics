declare module 'expo-router/entry';
declare module 'expo-router' {
  export { router, Router } from 'expo-router/build/imperative-api';
  export { Redirect, RedirectProps } from 'expo-router/build/link/Redirect';
  export { Stack } from 'expo-router/stack';
  export { Tabs } from 'expo-router/tabs';
  export { Link, LinkProps, WebAnchorProps } from 'expo-router/build/link/Link';
  export { 
    useRouter, 
    usePathname, 
    useSegments, 
    useGlobalSearchParams,
    useLocalSearchParams,
    useNavigation,
    useFocusEffect,
  } from 'expo-router/build/hooks';
  export * from 'expo-router/build/exports';
}
