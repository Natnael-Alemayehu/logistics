import { 
  View, 
  Text, 
  StyleSheet, 
  Pressable, 
  TextInput, 
  ActivityIndicator,
  Alert,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
} from 'react-native';
import { router, Redirect, Stack } from 'expo-router';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useAuthStore } from '@store/authStore';
import { useSettingsStore } from '@store/settingsStore';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

const ETHIOPIAN_PHONE_REGEX = /^(\+251|0)[1-9]\d{8}$/;

const loginSchema = z.object({
  phone: z
    .string()
    .min(1, 'Phone number is required')
    .refine((val) => ETHIOPIAN_PHONE_REGEX.test(val.replace(/\s|-/g, '')), {
      message: 'Enter a valid Ethiopian phone number (+251 or 0 prefix, 9 digits)',
    }),
  pin: z
    .string()
    .min(4, 'PIN must be at least 4 digits')
    .max(6, 'PIN cannot exceed 6 digits')
    .refine((val) => /^\d+$/.test(val), {
      message: 'PIN must contain only numbers',
    }),
});

type LoginFormData = z.infer<typeof loginSchema>;

export default function LoginScreen() {
  const { t } = useTranslation();
  const login = useAuthStore((state) => state.login);
  const isLoading = useAuthStore((state) => state.isLoading);
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const [error, setError] = useState<string | null>(null);
  const language = useSettingsStore((state) => state.language);
  const setLanguage = useSettingsStore((state) => state.setLanguage);

  const loginSchema = z.object({
    phone: z
      .string()
      .min(1, t('auth.phoneRequired'))
      .refine((val) => ETHIOPIAN_PHONE_REGEX.test(val.replace(/\s|-/g, '')), {
        message: t('auth.phoneInvalid'),
      }),
    pin: z
      .string()
      .min(4, t('auth.pinMinLength'))
      .max(6, t('auth.pinMaxLength'))
      .refine((val) => /^\d+$/.test(val), {
        message: t('auth.pinNumbersOnly'),
      }),
  });

  const {
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      phone: '',
      pin: '',
    },
  });

  const formatPhone = (phone: string): string => {
    const cleaned = phone.replace(/\s|-/g, '');
    if (cleaned.startsWith('0')) {
      return '+251' + cleaned.slice(1);
    }
    return cleaned.startsWith('+') ? cleaned : '+251' + cleaned;
  };

  const onSubmit = async (data: LoginFormData) => {
    setError(null);
    try {
      const formattedPhone = formatPhone(data.phone);
      await login(formattedPhone, data.pin);
      router.replace('/(main)');
    } catch (err) {
      const message = err instanceof Error ? err.message : t('auth.loginFailed');
      setError(message);
      Alert.alert(t('auth.loginFailed'), message);
    }
  };

  const toggleLanguage = () => {
    setLanguage(language === 'en' ? 'am' : 'en');
  };

  if (isAuthenticated) {
    return <Redirect href="/(main)" />;
  }

  return (
    <KeyboardAvoidingView 
      style={{ flex: 1 }} 
      behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
    >
      <ScrollView 
        style={styles.container} 
        contentContainerStyle={styles.contentContainer}
        keyboardShouldPersistTaps="handled"
      >
        <View style={styles.header}>
          <Text style={styles.logoIcon}>🚚</Text>
          <Text style={styles.title}>{t('auth.appTitle')}</Text>
          <Text style={styles.subtitle}>{t('auth.signInToContinue')}</Text>
        </View>

        <View style={styles.form}>
          <Controller
            control={control}
            name="phone"
            render={({ field: { onChange, onBlur, value } }) => (
              <View style={styles.inputContainer}>
                <Text style={styles.label}>{t('auth.phoneNumber')}</Text>
                <TextInput
                  style={[styles.input, errors.phone && styles.inputError]}
                  placeholder="0911234567 or +251911234567"
                  value={value}
                  onChangeText={onChange}
                  onBlur={onBlur}
                  autoCapitalize="none"
                  keyboardType="phone-pad"
                  autoComplete="tel"
                  textContentType="telephoneNumber"
                />
                {errors.phone && <Text style={styles.error}>{errors.phone.message}</Text>}
              </View>
            )}
          />

          <Controller
            control={control}
            name="pin"
            render={({ field: { onChange, onBlur, value } }) => (
              <View style={styles.inputContainer}>
                <Text style={styles.label}>{t('auth.pin')}</Text>
                <TextInput
                  style={[styles.input, errors.pin && styles.inputError]}
                  placeholder={t('auth.pinPlaceholder')}
                  value={value}
                  onChangeText={onChange}
                  onBlur={onBlur}
                  secureTextEntry
                  keyboardType="number-pad"
                  maxLength={6}
                />
                {errors.pin && <Text style={styles.error}>{errors.pin.message}</Text>}
              </View>
            )}
          />

          {error && (
            <View style={styles.errorContainer}>
              <Text style={styles.errorText}>{error}</Text>
            </View>
          )}

          <Pressable 
            style={[styles.button, isLoading && styles.buttonDisabled]} 
            onPress={handleSubmit(onSubmit)}
            disabled={isLoading}
          >
            {isLoading ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text style={styles.buttonText}>{t('auth.signIn')}</Text>
            )}
          </Pressable>

          <Pressable onPress={() => router.push('/forgot-pin')}>
            <Text style={styles.link}>{t('auth.forgotPin')}</Text>
          </Pressable>
        </View>

        <Pressable style={styles.languageToggle} onPress={toggleLanguage}>
          <Text style={styles.languageText}>{language === 'en' ? 'EN' : 'አማ'}</Text>
        </Pressable>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#fff',
  },
  contentContainer: {
    flexGrow: 1,
    padding: 20,
    justifyContent: 'center',
  },
  header: {
    alignItems: 'center',
    marginBottom: 40,
  },
  logoIcon: {
    fontSize: 64,
    marginBottom: 16,
  },
  title: {
    fontSize: 28,
    fontWeight: 'bold',
    color: '#1f2937',
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 16,
    color: '#6b7280',
  },
  form: {
    gap: 16,
  },
  inputContainer: {
    gap: 8,
  },
  label: {
    fontSize: 14,
    fontWeight: '500',
    color: '#374151',
  },
  input: {
    borderWidth: 1,
    borderColor: '#d1d5db',
    borderRadius: 12,
    padding: 16,
    fontSize: 16,
    backgroundColor: '#f9fafb',
  },
  inputError: {
    borderColor: '#ef4444',
    backgroundColor: '#fef2f2',
  },
  error: {
    color: '#ef4444',
    fontSize: 12,
  },
  errorContainer: {
    backgroundColor: '#fef2f2',
    padding: 12,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: '#fecaca',
  },
  errorText: {
    color: '#dc2626',
    fontSize: 14,
    textAlign: 'center',
  },
  button: {
    backgroundColor: '#2563eb',
    padding: 16,
    borderRadius: 12,
    alignItems: 'center',
    marginTop: 8,
  },
  buttonDisabled: {
    backgroundColor: '#93c5fd',
  },
  buttonText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: '600',
  },
  link: {
    color: '#2563eb',
    fontSize: 16,
    textAlign: 'center',
    marginTop: 16,
  },
  languageToggle: {
    position: 'absolute',
    top: 20,
    right: 20,
    backgroundColor: '#f3f4f6',
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 20,
    borderWidth: 1,
    borderColor: '#d1d5db',
  },
  languageText: {
    fontSize: 14,
    fontWeight: '600',
    color: '#374151',
  },
});