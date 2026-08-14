import React, { useState, useEffect, useCallback } from 'react';
import {
  View,
  StyleSheet,
  Text,
  TouchableOpacity,
  TextInput,
  ActivityIndicator,
  Linking,
  Platform,
} from 'react-native';
import * as Location from 'expo-location';
import { MapPin, AlertTriangle, CheckCircle, RefreshCw, Settings } from 'lucide-react-native';
import { colors, spacing } from '../../utils/theme';
import { useTranslation } from 'react-i18next';

export interface Coordinates {
  latitude: number;
  longitude: number;
}

interface LocationVerificationProps {
  destinationCoords: Coordinates;
  onVerified: (verified: boolean, mismatchMeters?: number) => void;
  onPermissionDenied?: () => void;
}

const LOCATION_THRESHOLD_METERS = 500;

const LocationVerification: React.FC<LocationVerificationProps> = ({
  destinationCoords,
  onVerified,
}) => {
  const { t } = useTranslation();
  const [isLoading, setIsLoading] = useState(true);
  const [currentCoords, setCurrentCoords] = useState<Coordinates | null>(null);
  const [distance, setDistance] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [proceedAnyway, setProceedAnyway] = useState(false);
  const [reason, setReason] = useState('');

  const calculateDistance = (
    lat1: number,
    lon1: number,
    lat2: number,
    lon2: number
  ): number => {
    const R = 6371000;
    const φ1 = (lat1 * Math.PI) / 180;
    const φ2 = (lat2 * Math.PI) / 180;
    const Δφ = ((lat2 - lat1) * Math.PI) / 180;
    const Δλ = ((lon2 - lon1) * Math.PI) / 180;

    const a =
      Math.sin(Δφ / 2) * Math.sin(Δφ / 2) +
      Math.cos(φ1) * Math.cos(φ2) * Math.sin(Δλ / 2) * Math.sin(Δλ / 2);
    const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));

    return R * c;
  };

  const checkLocation = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      const { status } = await Location.requestForegroundPermissionsAsync();
      if (status !== 'granted') {
        setError(t('pod.locationPermissionDenied'));
        onVerified(false, undefined);
        setIsLoading(false);
        return;
      }

      const location = await Location.getCurrentPositionAsync({
        accuracy: Location.Accuracy.High,
      });

      const current: Coordinates = {
        latitude: location.coords.latitude,
        longitude: location.coords.longitude,
      };

      setCurrentCoords(current);

      const calculatedDistance = calculateDistance(
        current.latitude,
        current.longitude,
        destinationCoords.latitude,
        destinationCoords.longitude
      );

      setDistance(calculatedDistance);

      const isWithinThreshold = calculatedDistance <= LOCATION_THRESHOLD_METERS;
      
      if (isWithinThreshold) {
        onVerified(true, calculatedDistance);
      } else {
        onVerified(false, calculatedDistance);
      }
    } catch (err) {
      setError(t('pod.locationError'));
      onVerified(false, undefined);
    } finally {
      setIsLoading(false);
    }
  }, [destinationCoords, onVerified, t]);

  useEffect(() => {
    checkLocation();
  }, [checkLocation]);

  const handleProceedAnyway = () => {
    if (reason.trim()) {
      onVerified(true, distance ?? undefined);
    }
  };

  const handleReattempt = () => {
    setProceedAnyway(false);
    setReason('');
    checkLocation();
  };

  if (isLoading) {
    return (
      <View style={styles.container}>
        <View style={styles.loadingContainer}>
          <ActivityIndicator size="small" color={colors.primary} />
          <Text style={styles.loadingText}>{t('pod.verifyingLocation')}</Text>
        </View>
      </View>
    );
  }

  if (error) {
    return (
      <View style={styles.container}>
        <View style={[styles.statusContainer, styles.errorContainer]}>
          <AlertTriangle size={24} color={colors.error} />
          <View style={styles.statusText}>
            <Text style={styles.statusTitle}>{t('pod.locationError')}</Text>
            <Text style={styles.statusMessage}>{error}</Text>
          </View>
        </View>
        <TouchableOpacity style={styles.reattemptButton} onPress={checkLocation}>
          <RefreshCw size={16} color={colors.primary} />
          <Text style={styles.reattemptText}>{t('pod.tryAgain')}</Text>
        </TouchableOpacity>
      </View>
    );
  }

  const isWithinThreshold = distance !== null && distance <= LOCATION_THRESHOLD_METERS;

  if (isWithinThreshold) {
    return (
      <View style={styles.container}>
        <View style={[styles.statusContainer, styles.successContainer]}>
          <CheckCircle size={24} color={colors.success} />
          <View style={styles.statusText}>
            <Text style={styles.statusTitle}>{t('pod.locationVerified')}</Text>
            <Text style={styles.statusMessage}>
              {t('pod.withinDistance', { distance: Math.round(distance!) })}
            </Text>
          </View>
        </View>
      </View>
    );
  }

  if (proceedAnyway) {
    return (
      <View style={styles.container}>
        <View style={[styles.statusContainer, styles.warningContainer]}>
          <AlertTriangle size={24} color={colors.warning} />
          <View style={styles.statusText}>
            <Text style={styles.statusTitle}>{t('pod.locationMismatch')}</Text>
            <Text style={styles.statusMessage}>
              {distance !== null && t('pod.distanceFromDestination', { distance: Math.round(distance) })}
            </Text>
          </View>
        </View>
        
        <View style={styles.reasonContainer}>
          <Text style={styles.reasonLabel}>{t('pod.mismatchReason')} *</Text>
          <TextInput
            style={styles.reasonInput}
            placeholder={t('pod.mismatchReasonPlaceholder')}
            value={reason}
            onChangeText={setReason}
            multiline
            numberOfLines={3}
            placeholderTextColor={colors.textSecondary}
          />
          
          <View style={styles.actionButtons}>
            <TouchableOpacity
              style={styles.secondaryButton}
              onPress={handleReattempt}
            >
              <Text style={styles.secondaryButtonText}>{t('pod.reattempt')}</Text>
            </TouchableOpacity>
            
            <TouchableOpacity
              style={[
                styles.primaryButton,
                !reason.trim() && styles.disabledButton,
              ]}
              onPress={handleProceedAnyway}
              disabled={!reason.trim()}
            >
              <Text style={styles.primaryButtonText}>{t('pod.proceedAnyway')}</Text>
            </TouchableOpacity>
          </View>
        </View>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <View style={[styles.statusContainer, styles.warningContainer]}>
        <MapPin size={24} color={colors.warning} />
        <View style={styles.statusText}>
          <Text style={styles.statusTitle}>{t('pod.locationMismatch')}</Text>
          <Text style={styles.statusMessage}>
            {distance !== null
              ? t('pod.distanceWithThreshold', { distance: Math.round(distance), threshold: LOCATION_THRESHOLD_METERS })
              : t('pod.unableToCalculate')}
          </Text>
        </View>
      </View>
      
      <View style={styles.actionButtons}>
        <TouchableOpacity style={styles.reattemptButton} onPress={handleReattempt}>
          <RefreshCw size={16} color={colors.primary} />
          <Text style={styles.reattemptText}>{t('pod.reattempt')}</Text>
        </TouchableOpacity>
        
        <TouchableOpacity
          style={styles.proceedButton}
          onPress={() => setProceedAnyway(true)}
        >
          <Text style={styles.proceedText}>{t('pod.proceedAnyway')}</Text>
        </TouchableOpacity>
      </View>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    width: '100%',
  },
  loadingContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    padding: spacing.lg,
  },
  loadingText: {
    marginLeft: spacing.sm,
    fontSize: 14,
    color: colors.textSecondary,
  },
  statusContainer: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    padding: spacing.lg,
    borderRadius: 12,
    gap: spacing.md,
  },
  successContainer: {
    backgroundColor: '#dcfce7',
  },
  warningContainer: {
    backgroundColor: '#fef3c7',
  },
  errorContainer: {
    backgroundColor: '#fee2e2',
  },
  statusText: {
    flex: 1,
  },
  statusTitle: {
    fontSize: 16,
    fontWeight: '600',
    color: colors.text,
    marginBottom: spacing.xs,
  },
  statusMessage: {
    fontSize: 14,
    color: colors.textSecondary,
  },
  actionButtons: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginTop: spacing.md,
    gap: spacing.md,
  },
  reattemptButton: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: spacing.sm,
    paddingHorizontal: spacing.md,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.primary,
    gap: spacing.xs,
  },
  reattemptText: {
    color: colors.primary,
    fontSize: 14,
    fontWeight: '500',
  },
  proceedButton: {
    paddingVertical: spacing.sm,
    paddingHorizontal: spacing.md,
    borderRadius: 8,
    backgroundColor: colors.warning,
  },
  proceedText: {
    color: colors.surface,
    fontSize: 14,
    fontWeight: '500',
  },
  reasonContainer: {
    marginTop: spacing.md,
    padding: spacing.lg,
    backgroundColor: colors.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: colors.border,
  },
  reasonLabel: {
    fontSize: 14,
    fontWeight: '500',
    color: colors.text,
    marginBottom: spacing.sm,
  },
  reasonInput: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: spacing.md,
    fontSize: 16,
    color: colors.text,
    minHeight: 80,
    textAlignVertical: 'top',
    backgroundColor: colors.background,
  },
  secondaryButton: {
    flex: 1,
    paddingVertical: spacing.md,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.border,
    alignItems: 'center',
  },
  secondaryButtonText: {
    color: colors.textSecondary,
    fontSize: 14,
    fontWeight: '500',
  },
  primaryButton: {
    flex: 1,
    paddingVertical: spacing.md,
    borderRadius: 8,
    backgroundColor: colors.primary,
    alignItems: 'center',
  },
  disabledButton: {
    backgroundColor: colors.border,
  },
  primaryButtonText: {
    color: colors.surface,
    fontSize: 14,
    fontWeight: '500',
  },
});

export default LocationVerification;
export { LocationVerificationProps };
