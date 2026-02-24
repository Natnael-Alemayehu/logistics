import { View, Text, StyleSheet } from 'react-native';
import { colors, spacing } from '../../utils/theme';
import { useTranslation } from 'react-i18next';

export function EmptyShipments() {
  const { t } = useTranslation();
  
  return (
    <View style={styles.container}>
      <View style={styles.iconContainer}>
        <Text style={styles.icon}>📦</Text>
      </View>
      <Text style={styles.title}>{t('shipments.noShipments')}</Text>
      <Text style={styles.subtitle}>
        {t('shipments.pullToRefresh')}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: spacing.xxl,
  },
  iconContainer: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: colors.background,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: spacing.lg,
  },
  icon: {
    fontSize: 40,
  },
  title: {
    fontSize: 18,
    fontWeight: '600',
    color: colors.text,
    marginBottom: spacing.sm,
  },
  subtitle: {
    fontSize: 14,
    color: colors.textSecondary,
    textAlign: 'center',
  },
});
