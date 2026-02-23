import React, { useState, useCallback } from 'react';
import {
  View,
  StyleSheet,
  TouchableOpacity,
  Text,
  Image,
  Alert,
  Dimensions,
} from 'react-native';
import { CameraView, CameraType, useCameraPermissions } from 'expo-camera';
import * as Location from 'expo-location';
import { X, Camera } from 'lucide-react-native';
import { colors, spacing } from '../../utils/theme';

const { width } = Dimensions.get('window');
const THUMBNAIL_SIZE = (width - spacing.xl * 2 - spacing.md * 2) / 3;

interface PhotoData {
  uri: string;
  latitude?: number;
  longitude?: number;
  timestamp: Date;
}

interface PhotoCaptureProps {
  photos: string[];
  onAddPhoto: () => void;
  onRemovePhoto: (index: number) => void;
  maxPhotos?: number;
}

const PhotoCapture: React.FC<PhotoCaptureProps> = ({
  photos,
  onAddPhoto,
  onRemovePhoto,
  maxPhotos = 3,
}) => {
  const canAddPhoto = photos.length < maxPhotos;
  const remainingSlots = maxPhotos - photos.length;

  return (
    <View style={styles.container}>
      <View style={styles.photoGrid}>
        {photos.map((photoUri, index) => (
          <View key={index} style={styles.photoContainer}>
            <Image
              source={{ uri: photoUri }}
              style={styles.thumbnail}
              resizeMode="cover"
            />
            <TouchableOpacity
              style={styles.removeButton}
              onPress={() => onRemovePhoto(index)}
              hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
            >
              <X size={14} color={colors.surface} />
            </TouchableOpacity>
          </View>
        ))}
        
        {canAddPhoto && (
          <TouchableOpacity style={styles.addButton} onPress={onAddPhoto}>
            <View style={styles.addButtonContent}>
              <Camera size={24} color={colors.textSecondary} />
              <Text style={styles.addButtonText}>Add Photo</Text>
            </View>
          </TouchableOpacity>
        )}
      </View>
      
      <Text style={styles.hint}>
        {photos.length === 0 
          ? `Add up to ${maxPhotos} photos (optional)`
          : `${remainingSlots} photo${remainingSlots !== 1 ? 's' : ''} remaining`
        }
      </Text>
    </View>
  );
};

interface CameraModalProps {
  visible: boolean;
  onCapture: (photo: PhotoData) => void;
  onClose: () => void;
}

const CameraModal: React.FC<CameraModalProps> = ({
  visible,
  onCapture,
  onClose,
}) => {
  const [permission, requestPermission] = useCameraPermissions();
  const [locationPermission, setLocationPermission] = useState<boolean>(false);
  const [facing, setFacing] = useState<CameraType>('back');

  const requestPermissions = useCallback(async () => {
    const { status: locationStatus } = await Location.requestForegroundPermissionsAsync();
    setLocationPermission(locationStatus === 'granted');
  }, []);

  const handleCapture = useCallback(async () => {
    // Implementation would use camera ref to take picture
    // For now, this is a placeholder that would be connected to actual camera
  }, []);

  if (!permission) {
    return null;
  }

  if (!permission.granted) {
    return (
      <View style={styles.permissionContainer}>
        <Camera size={48} color={colors.textSecondary} />
        <Text style={styles.permissionTitle}>Camera Access Required</Text>
        <Text style={styles.permissionText}>
          Please grant camera access to capture delivery photos
        </Text>
        <TouchableOpacity style={styles.permissionButton} onPress={requestPermission}>
          <Text style={styles.permissionButtonText}>Grant Permission</Text>
        </TouchableOpacity>
      </View>
    );
  }

  return null;
};

const styles = StyleSheet.create({
  container: {
    width: '100%',
  },
  photoGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.md,
  },
  photoContainer: {
    width: THUMBNAIL_SIZE,
    height: THUMBNAIL_SIZE,
    position: 'relative',
  },
  thumbnail: {
    width: '100%',
    height: '100%',
    borderRadius: 8,
    backgroundColor: colors.background,
  },
  removeButton: {
    position: 'absolute',
    top: -spacing.xs,
    right: -spacing.xs,
    width: 24,
    height: 24,
    borderRadius: 12,
    backgroundColor: colors.error,
    justifyContent: 'center',
    alignItems: 'center',
  },
  addButton: {
    width: THUMBNAIL_SIZE,
    height: THUMBNAIL_SIZE,
    borderWidth: 2,
    borderColor: colors.border,
    borderStyle: 'dashed',
    borderRadius: 8,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: colors.background,
  },
  addButtonContent: {
    alignItems: 'center',
    gap: spacing.xs,
  },
  addButtonText: {
    fontSize: 12,
    color: colors.textSecondary,
    fontWeight: '500',
  },
  hint: {
    fontSize: 12,
    color: colors.textSecondary,
    marginTop: spacing.sm,
    textAlign: 'center',
  },
  permissionContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: spacing.xxl,
    backgroundColor: colors.background,
  },
  permissionTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: colors.text,
    marginTop: spacing.lg,
    marginBottom: spacing.sm,
  },
  permissionText: {
    fontSize: 14,
    color: colors.textSecondary,
    textAlign: 'center',
    marginBottom: spacing.lg,
  },
  permissionButton: {
    backgroundColor: colors.primary,
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.xl,
    borderRadius: 8,
  },
  permissionButtonText: {
    color: colors.surface,
    fontSize: 16,
    fontWeight: '600',
  },
});

export default PhotoCapture;
export { PhotoCaptureProps, PhotoData };
