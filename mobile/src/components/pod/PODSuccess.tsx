import React, { useEffect, useRef } from 'react';
import {
  View,
  StyleSheet,
  Text,
  Animated,
  Easing,
} from 'react-native';
import { CheckCircle, Package } from 'lucide-react-native';
import { colors, spacing } from '../../utils/theme';
import Button from '../ui/Button';
import { useTranslation } from 'react-i18next';

interface PODSuccessProps {
  shipmentId: string;
  trackingNumber: string;
  onDone: () => void;
}

const PODSuccess: React.FC<PODSuccessProps> = ({
  shipmentId,
  trackingNumber,
  onDone,
}) => {
  const { t } = useTranslation();
  const scaleAnim = useRef(new Animated.Value(0)).current;
  const fadeAnim = useRef(new Animated.Value(0)).current;
  const checkmarkAnim = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    Animated.sequence([
      Animated.timing(scaleAnim, {
        toValue: 1,
        duration: 400,
        easing: Easing.out(Easing.back(1.5)),
        useNativeDriver: true,
      }),
      Animated.parallel([
        Animated.timing(fadeAnim, {
          toValue: 1,
          duration: 300,
          useNativeDriver: true,
        }),
        Animated.timing(checkmarkAnim, {
          toValue: 1,
          duration: 500,
          easing: Easing.out(Easing.quad),
          useNativeDriver: true,
        }),
      ]),
    ]).start();
  }, [scaleAnim, fadeAnim, checkmarkAnim]);

  const checkmarkScale = checkmarkAnim.interpolate({
    inputRange: [0, 0.5, 1],
    outputRange: [0, 1.2, 1],
  });

  return (
    <View style={styles.container}>
      <View style={styles.content}>
        <Animated.View
          style={[
            styles.iconContainer,
            { transform: [{ scale: scaleAnim }] },
          ]}
        >
          <Animated.View
            style={{ transform: [{ scale: checkmarkScale }] }}
          >
            <CheckCircle size={80} color={colors.success} strokeWidth={2} />
          </Animated.View>
        </Animated.View>

        <Animated.View
          style={[
            styles.textContainer,
            { opacity: fadeAnim },
          ]}
        >
          <Text style={styles.title}>{t('pod.deliveryComplete')}</Text>
          <Text style={styles.message}>
            {t('pod.podSubmitted')}
          </Text>
        </Animated.View>

        <Animated.View
          style={[
            styles.trackingContainer,
            { opacity: fadeAnim },
          ]}
        >
          <View style={styles.trackingCard}>
            <Package size={24} color={colors.primary} />
            <View style={styles.trackingInfo}>
              <Text style={styles.trackingLabel}>{t('shipments.trackingNumber')}</Text>
              <Text style={styles.trackingNumber}>{trackingNumber}</Text>
            </View>
          </View>
        </Animated.View>
      </View>

      <Animated.View style={[styles.buttonContainer, { opacity: fadeAnim }]}>
        <Button
          title={t('pod.viewShipments')}
          onPress={onDone}
          fullWidth
          size="lg"
        />
      </Animated.View>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.background,
    paddingHorizontal: spacing.xl,
    justifyContent: 'space-between',
    paddingVertical: spacing.xxl,
  },
  content: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  iconContainer: {
    width: 120,
    height: 120,
    borderRadius: 60,
    backgroundColor: '#dcfce7',
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: spacing.xxl,
  },
  textContainer: {
    alignItems: 'center',
    marginBottom: spacing.xxl,
  },
  title: {
    fontSize: 28,
    fontWeight: '700',
    color: colors.text,
    marginBottom: spacing.sm,
    textAlign: 'center',
  },
  message: {
    fontSize: 16,
    color: colors.textSecondary,
    textAlign: 'center',
    maxWidth: 280,
  },
  trackingContainer: {
    width: '100%',
    maxWidth: 320,
  },
  trackingCard: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: colors.surface,
    padding: spacing.lg,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: colors.border,
    gap: spacing.md,
  },
  trackingInfo: {
    flex: 1,
  },
  trackingLabel: {
    fontSize: 12,
    color: colors.textSecondary,
    marginBottom: spacing.xs,
  },
  trackingNumber: {
    fontSize: 16,
    fontWeight: '600',
    color: colors.text,
  },
  buttonContainer: {
    width: '100%',
    maxWidth: 320,
    alignSelf: 'center',
  },
});

export default PODSuccess;
export { PODSuccessProps };
